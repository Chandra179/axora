package crawler

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"axora/pkg/chunking"
	"axora/repository"

	"github.com/gocolly/colly/v2"
	"golang.org/x/net/proxy"
)

type Worker struct {
	collector       *colly.Collector
	chunker         chunking.ChunkingClient
	crawlVectorRepo repository.CrawlVectorRepo
	extractor       *ContentExtractor
	visitedURL      map[string]int
	maxURLVisits    int
	mutex           sync.RWMutex
	httpClient      http.Client
}

// getPublicIP makes a request to check the current public IP being used
func (w *Worker) getPublicIP() string {
	// Try multiple IP checking services
	services := []string{
		"https://httpbin.org/ip",
		"https://api.ipify.org?format=text",
		"https://icanhazip.com",
	}

	for _, service := range services {
		req, err := http.NewRequest("GET", service, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Axora-Crawler/1.0")

		resp, err := w.httpClient.Do(req)
		if err != nil {
			log.Printf("[IP_CHECK_ERROR] Failed to check IP via %s: %v", service, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				continue
			}

			ipStr := strings.TrimSpace(string(body))
			// For httpbin.org/ip, extract IP from JSON response
			if strings.Contains(service, "httpbin") && strings.Contains(ipStr, "origin") {
				// Parse JSON-like response: {"origin": "1.2.3.4"}
				start := strings.Index(ipStr, `"`) + 1
				end := strings.LastIndex(ipStr, `"`)
				if start > 0 && end > start {
					ipStr = ipStr[start:end]
					if strings.Contains(ipStr, "origin") {
						parts := strings.Split(ipStr, ": ")
						if len(parts) > 1 {
							ipStr = strings.Trim(parts[1], `"`)
						}
					}
				}
			}

			log.Printf("[IP_CHECK_SUCCESS] Current public IP: %s (via %s)", ipStr, service)
			return ipStr
		}
	}

	log.Printf("[IP_CHECK_FAILED] Could not determine public IP")
	return "unknown"
}

func NewWorker(crawlVectorRepo repository.CrawlVectorRepo, extractor *ContentExtractor,
	chunker chunking.ChunkingClient) *Worker {

	dialer, _ := proxy.SOCKS5("tcp", "axora-tor:9050", nil, proxy.Direct)
	transport := &http.Transport{Dial: dialer.Dial}
	client := &http.Client{Transport: transport}

	c := colly.NewCollector(
		colly.UserAgent("Axora-Crawler/1.0"),
		colly.MaxDepth(3),
		colly.AllowURLRevisit(),
		colly.Async(true),
		// colly.Debugger(&debug.LogDebugger{}),
	)
	c.WithTransport(transport)
	c.SetClient(client)
	c.SetRequestTimeout(300 * time.Second)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       5 * time.Second,
	})

	worker := &Worker{
		collector:       c,
		chunker:         chunker,
		crawlVectorRepo: crawlVectorRepo,
		extractor:       extractor,
		visitedURL:      make(map[string]int),
		maxURLVisits:    3,
		httpClient:      *client,
	}

	// Check and log the current IP being used
	log.Printf("[WORKER_INIT] Initializing worker with Tor proxy (axora-tor:9050)")
	currentIP := worker.getPublicIP()
	log.Printf("[WORKER_INIT] Worker initialized - Using IP: %s", currentIP)

	return worker
}

// isValidDownloadURL validates URL according to the specification
func (w *Worker) isValidDownloadURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	if u.Scheme != "https" {
		return false
	}

	allowedPaths := []string{"/index.php", "/edition.php", "/ads.php", "/get.php"}
	pathValid := false
	for _, allowedPath := range allowedPaths {
		if u.Path == allowedPath {
			pathValid = true
			break
		}
	}
	if !pathValid {
		return false
	}

	allowedParams := map[string]bool{
		"req":          true,
		"id":           true,
		"md5":          true,
		"downloadname": true,
		"key":          true,
	}
	for param := range u.Query() {
		if !allowedParams[param] {
			return false
		}
	}

	return true
}

func (w *Worker) Crawl(ctx context.Context, urls []string) {
	// Check IP before starting crawl
	currentIP := w.getPublicIP()
	log.Printf("[CRAWL_START] Starting crawl with IP: %s", currentIP)

	w.collector.OnRequest(func(r *colly.Request) {
		w.mutex.Lock()
		defer w.mutex.Unlock()
		w.visitedURL[r.URL.String()]++
		log.Printf("[REQUEST] Visiting: %s (visit count: %d) [IP: %s]", r.URL.String(), w.visitedURL[r.URL.String()], currentIP)
	})

	w.collector.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		absoluteURL := e.Request.AbsoluteURL(link)
		u, err := url.ParseRequestURI(absoluteURL)
		if err != nil {
			return
		}
		if u.Scheme != "https" {
			return
		}
		if u.Host == "" {
			return
		}
		if !w.isValidDownloadURL(absoluteURL) {
			return
		}

		w.mutex.Lock()
		defer w.mutex.Unlock()
		currentVisits := w.visitedURL[absoluteURL]
		if currentVisits >= w.maxURLVisits {
			log.Printf("[VISIT_COUNTER] BLOCKING URL: %s - exceeded max visits %d", absoluteURL, w.maxURLVisits)
			return
		}

		log.Printf("[URL_VALIDATION] Valid download URL found: %s [IP: %s]", absoluteURL, currentIP)
		e.Request.Visit(absoluteURL)
	})

	w.collector.OnError(func(r *colly.Response, err error) {
		if r == nil {
			log.Printf("[HTTP_ERROR] Request failed: %v [IP: %s]", err, currentIP)
			return
		}

		log.Printf("[HTTP_ERROR] URL: %s - Status: %d - Error: %v [IP: %s]", r.Request.URL, r.StatusCode, err, currentIP)

		if r.StatusCode >= 500 || r.StatusCode == 0 {
			retryCount := r.Request.Ctx.GetAny("retryCount")
			if retryCount == nil {
				r.Ctx.Put("retryCount", 1)
			}
			rc := r.Ctx.GetAny("retryCount").(int)
			if rc < 3 {
				log.Printf("[RETRY] 502 from %s, retry %d/3 [IP: %s]", r.Request.URL, rc+1, currentIP)

				// clone context with incremented retry count
				r.Ctx.Put("retryCount", rc+1)

				time.Sleep(time.Duration(rc+1) * time.Second) // backoff
				err := w.collector.Request("GET", r.Request.URL.String(), nil, r.Ctx, nil)
				if err != nil {
					log.Printf("[RETRY_ERROR] failed to resubmit %s: %v [IP: %s]", r.Request.URL, err, currentIP)
				}
			} else {
				log.Printf("[RETRY_FAILED] Max retries reached for %s [IP: %s]", r.Request.URL, currentIP)
			}
		}
	})

	w.collector.OnResponse(func(r *colly.Response) {
		log.Printf("[RESPONSE] URL: %s - Status: %d - Size: %d bytes [IP: %s]", r.Request.URL.String(), r.StatusCode, len(r.Body), currentIP)

		contentType := r.Headers.Get("Content-Type")
		contentLength := r.Headers.Get("Content-Length")
		contentDisposition := r.Headers.Get("Content-Disposition")

		log.Printf("[RESPONSE_HEADERS] Content-Type: %s, Content-Length: %s [IP: %s]", contentType, contentLength, currentIP)

		if !strings.Contains(strings.ToLower(contentDisposition), "attachment") {
			return
		}
		if contentType != "application/octet-stream" {
			return
		}
		filename := "download.bin"
		if strings.Contains(contentDisposition, "filename=") {
			parts := strings.Split(contentDisposition, "filename=")
			if len(parts) > 1 {
				filename = strings.Trim(parts[1], "\"")
			}
		}

		u := r.Request.URL
		q := u.Query()
		md5hash := q.Get("md5")
		savePath := filepath.Join("/app/downloads", filename)

		go func(url, path, fname, hash string) {
			log.Printf("[DOWNLOAD_START] Starting download: %s [IP: %s]", fname, currentIP)
			err := w.downloadFile(u.String(), savePath)
			if err != nil {
				log.Printf("[DOWNLOAD_ERROR] %s -> %v [IP: %s]", filename, err, currentIP)
			} else {
				log.Printf("[DOWNLOAD_SUCCESS] Saved %s (md5=%s) [IP: %s]", savePath, md5hash, currentIP)
			}

		}(u.String(), savePath, filename, md5hash)
	})

	for _, url := range urls {
		err := w.collector.Visit(url)
		if err != nil {
			log.Printf("Failed to visit %s: %v [IP: %s]", url, err, currentIP)
		}
	}

	w.collector.Wait()
	log.Printf("[CRAWL_COMPLETE] Crawling completed [IP: %s]", currentIP)
}

func (w *Worker) downloadFile(url, savePath string) error {
	// Check IP before download to ensure consistency
	currentIP := w.getPublicIP()
	log.Printf("[DOWNLOAD_URL] %s [IP: %s]", url, currentIP)

	// Ensure the directory exists
	dir := filepath.Dir(savePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Check if file already exists and get current size
	var startPos int64 = 0
	if fi, err := os.Stat(savePath); err == nil {
		startPos = fi.Size()
		log.Printf("[DOWNLOAD_RESUME] Resuming download from byte %d [IP: %s]", startPos, currentIP)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("[ERROR_CREATE_REQ] %v", err)
	}
	req.Header.Set("User-Agent", "GoDownloader/1.0")

	// If resuming, set Range header
	if startPos > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startPos))
		log.Printf("[DOWNLOAD_RANGE] Requesting bytes %d- [IP: %s]", startPos, currentIP)
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		log.Printf("[ERROR_GET_REQ] %v [IP: %s]", err, currentIP)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	log.Printf("[DOWNLOAD_RESPONSE] Status: %d, Content-Length: %s [IP: %s]",
		resp.StatusCode, resp.Header.Get("Content-Length"), currentIP)

	// Open file (append if resuming)
	var out *os.File
	if startPos > 0 {
		out, err = os.OpenFile(savePath, os.O_WRONLY|os.O_APPEND, 0644)
	} else {
		out, err = os.Create(savePath)
	}
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("copy error: %w [IP: %s]", err, currentIP)
	}

	log.Printf("[DOWNLOAD_COMPLETE] Successfully downloaded to %s [IP: %s]", savePath, currentIP)
	return nil
}
