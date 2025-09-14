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
	w.collector.OnRequest(func(r *colly.Request) {
		w.mutex.Lock()
		defer w.mutex.Unlock()
		w.visitedURL[r.URL.String()]++
		log.Printf("[REQUEST] Visiting: %s (visit count: %d)", r.URL.String(), w.visitedURL[r.URL.String()])
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

		log.Printf("[URL_VALIDATION] Valid download URL found: %s", absoluteURL)
		e.Request.Visit(absoluteURL)
	})

	w.collector.OnError(func(r *colly.Response, err error) {
		if r == nil {
			log.Printf("[HTTP_ERROR] Request failed: %v", err)
			return
		}

		log.Printf("[HTTP_ERROR] URL: %s - Status: %d - Error: %v", r.Request.URL, r.StatusCode, err)

		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded") {
			log.Printf("[TIMEOUT] Timeout occurred for URL: %s. This may be due to Tor network latency or slow server response.", r.Request.URL)
			log.Printf("[TOR] Consider rotating IP if timeouts persist")
		}

		if r.StatusCode == 429 {
			log.Printf("[RATE_LIMIT] Rate limited by server: %s", r.Request.URL)
		} else if r.StatusCode >= 500 {
			log.Printf("[SERVER_ERROR] Server error for URL: %s", r.Request.URL)
		}
	})

	w.collector.OnResponse(func(r *colly.Response) {
		log.Printf("[RESPONSE] URL: %s - Status: %d - Size: %d bytes", r.Request.URL.String(), r.StatusCode, len(r.Body))

		contentType := r.Headers.Get("Content-Type")
		contentLength := r.Headers.Get("Content-Length")
		contentDisposition := r.Headers.Get("Content-Disposition")

		log.Printf("[RESPONSE_HEADERS] Content-Type: %s, Content-Length: %s", contentType, contentLength)

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
		savePath := filepath.Join("./downloads", filename)
		err := w.downloadFile(u.String(), savePath)
		if err != nil {
			log.Printf("[DOWNLOAD_ERROR] %s -> %v", filename, err)
		} else {
			log.Printf("[DOWNLOAD_SUCCESS] Saved %s (md5=%s)", savePath, md5hash)
		}
	})

	for _, url := range urls {
		err := w.collector.Visit(url)
		if err != nil {
			log.Printf("Failed to visit %s: %v", url, err)
		}
	}

	w.collector.Wait()
}

func (w *Worker) downloadFile(url, savePath string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	// IMPORTANT: Don't add Range header, let server send full file
	req.Header.Set("User-Agent", "GoDownloader/1.0")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	out, err := os.Create(savePath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
