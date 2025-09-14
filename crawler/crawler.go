package crawler

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"axora/pkg/chunking"
	"axora/pkg/tor"
	"axora/repository"

	"github.com/gocolly/colly/v2"
)

type Worker struct {
	collector       *colly.Collector
	chunker         chunking.ChunkingClient
	crawlVectorRepo repository.CrawlVectorRepo
	extractor       *ContentExtractor
	visitedURL      map[string]int
	maxURLVisits    int
	mutex           sync.RWMutex
	downloadManager *DownloadManager
	torClient       *tor.TorClient
}

func NewWorker(crawlVectorRepo repository.CrawlVectorRepo, extractor *ContentExtractor,
	chunker chunking.ChunkingClient, torClient *tor.TorClient) *Worker {

	c := colly.NewCollector(
		colly.UserAgent("Axora-Crawler/1.0"),
		colly.MaxDepth(3),
		colly.AllowURLRevisit(),
		colly.Async(true),
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
		// colly.Debugger(&debug.LogDebugger{}),
	)

	if torClient != nil {
		transport := &http.Transport{
			DialContext:           torClient.GetDialContextWithRetry(3, 3*time.Second),
			DisableKeepAlives:     true,
			MaxIdleConns:          0,
			MaxIdleConnsPerHost:   0,
			MaxConnsPerHost:       2, // Allow 2 concurrent connections
			IdleConnTimeout:       0,
			ResponseHeaderTimeout: 180 * time.Second, // 3 minutes for slow responses
			TLSHandshakeTimeout:   60 * time.Second,
		}

		c.WithTransport(transport)
		log.Println("Colly configured to use Tor proxy")
	}

	c.SetRequestTimeout(300 * time.Second) // 5 minutes total timeout
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,               // Single request at a time to be respectful
		Delay:       5 * time.Second, // Longer delay between requests
		RandomDelay: 2 * time.Second, // Add random delay
	})

	worker := &Worker{
		collector:       c,
		chunker:         chunker,
		crawlVectorRepo: crawlVectorRepo,
		extractor:       extractor,
		visitedURL:      make(map[string]int),
		maxURLVisits:    3,
		downloadManager: NewDownloadManager(torClient),
		torClient:       torClient,
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

// isDownloadResponse checks if the response is a download
func (w *Worker) isDownloadResponse(r *colly.Response) bool {
	contentDisposition := r.Headers.Get("Content-Disposition")
	contentType := r.Headers.Get("Content-Type")

	if strings.Contains(strings.ToLower(contentDisposition), "attachment") {
		return true
	}

	if contentType == "application/octet-stream" {
		return true
	}

	return false
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

		// Check if it's a timeout error
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded") {
			log.Printf("[TIMEOUT] Timeout occurred for URL: %s. This may be due to Tor network latency or slow server response.", r.Request.URL)
			log.Printf("[TOR] Consider rotating IP if timeouts persist")
		}

		// Check for specific HTTP errors
		if r.StatusCode == 429 {
			log.Printf("[RATE_LIMIT] Rate limited by server: %s", r.Request.URL)
			// Consider longer delays
		} else if r.StatusCode >= 500 {
			log.Printf("[SERVER_ERROR] Server error for URL: %s", r.Request.URL)
		}
	})

	w.collector.OnResponse(func(r *colly.Response) {
		log.Printf("[RESPONSE] URL: %s - Status: %d - Size: %d bytes", r.Request.URL.String(), r.StatusCode, len(r.Body))

		// Log response headers for debugging
		contentType := r.Headers.Get("Content-Type")
		contentLength := r.Headers.Get("Content-Length")
		log.Printf("[RESPONSE_HEADERS] Content-Type: %s, Content-Length: %s", contentType, contentLength)

		if !w.isDownloadResponse(r) {
			return
		}

		rurl := r.Request.URL.String()
		log.Printf("[DOWNLOAD] Download detected for URL: %s", rurl)
		u, _ := url.Parse(rurl)
		expectedMD5 := u.Query().Get("md5")
		filename := "download_" + strings.ReplaceAll(rurl, "/", "_")

		go func() {
			err := w.downloadManager.Download(rurl, filename, expectedMD5)
			if err != nil {
				log.Printf("[DOWNLOAD] Failed to download %s: %v", rurl, err)
			} else {
				log.Printf("[DOWNLOAD] Successfully downloaded: %s", filename)
			}
		}()
	})

	for _, url := range urls {
		err := w.collector.Visit(url)
		if err != nil {
			log.Printf("Failed to visit %s: %v", url, err)
		}
	}

	w.collector.Wait()
}
