package crawler

import (
	"context"
	"log"
	"net/url"
	"strings"
	"sync"

	"axora/pkg/chunking"
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
}

func NewWorker(crawlVectorRepo repository.CrawlVectorRepo, extractor *ContentExtractor,
	chunker chunking.ChunkingClient) *Worker {
	c := colly.NewCollector(
		colly.UserAgent("Axora-Crawler/1.0"),
		colly.MaxDepth(3),
		colly.AllowURLRevisit(),
		colly.Async(true),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 3,
	})

	worker := &Worker{
		collector:       c,
		chunker:         chunker,
		crawlVectorRepo: crawlVectorRepo,
		extractor:       extractor,
		visitedURL:      make(map[string]int),
		maxURLVisits:    10,
		downloadManager: NewDownloadManager(),
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
			log.Printf("[URL_VALIDATION] Invalid query parameter: %s in URL: %s", param, rawURL)
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
			log.Printf("[URL_VALIDATION] Invalid download URL: %s", absoluteURL)
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
		if r != nil {
			log.Printf("[HTTP_ERROR] URL: %s - Status: %d - Error: %v", r.Request.URL, r.StatusCode, err)
		} else {
			log.Printf("[HTTP_ERROR] Request failed before response: %v", err)
		}
	})

	w.collector.OnResponse(func(r *colly.Response) {
		if w.isDownloadResponse(r) {
			log.Printf("[DOWNLOAD] Download detected for URL: %s", r.Request.URL.String())

			// Extract MD5 from URL if present
			u, _ := url.Parse(r.Request.URL.String())
			expectedMD5 := u.Query().Get("md5")
			filename := u.Query().Get("downloadname")

			if filename == "" {
				// Try to extract filename from Content-Disposition
				contentDisposition := r.Headers.Get("Content-Disposition")
				if strings.Contains(contentDisposition, "filename=") {
					parts := strings.Split(contentDisposition, "filename=")
					if len(parts) > 1 {
						filename = strings.Trim(strings.Split(parts[1], ";")[0], `"`)
					}
				}
			}

			if filename == "" {
				filename = "download_" + strings.ReplaceAll(r.Request.URL.String(), "/", "_")
			}

			// Start download using download manager
			go func() {
				err := w.downloadManager.Download(r.Request.URL.String(), filename, expectedMD5)
				if err != nil {
					log.Printf("[DOWNLOAD] Failed to download %s: %v", r.Request.URL.String(), err)
				} else {
					log.Printf("[DOWNLOAD] Successfully downloaded: %s", filename)
				}
			}()
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

// func (w *Worker) scrape(ctx context.Context) {
// 	w.collector.OnScraped(func(r *colly.Response) {
// 		url := r.Request.URL.String()
// 		content, err := w.extractor.ExtractText(string(r.Body), r.Request.URL)
// 		if err != nil {
// 			log.Printf("err extracting text: %v", err)
// 			return
// 		}
// 		if content.IsBoilerplate {
// 			return
// 		}

// 		chunks, err := w.chunker.ChunkText(content.Text)
// 		if err != nil {
// 			log.Printf("Error chunking text for URL: %s Error: %v", url, err)
// 			return
// 		}

// 		g, ctx := errgroup.WithContext(ctx)
// 		g.SetLimit(2)

// 		for _, chunk := range chunks {
// 			g.Go(func(c chunking.ChunkOutput) func() error {
// 				return func() error {
// 					isRelevant, score, err := w.relevanceFilter.IsContentRelevant(c.Text)
// 					if err != nil {
// 						return fmt.Errorf("err checking relevance: %v", err)
// 					}
// 					if !isRelevant {
// 						return nil
// 					}
// 					log.Printf("URL: %s Score: %.3f", url, score)
// 					return w.crawlVectorRepo.InsertOne(ctx, &repository.CrawlVectorDoc{
// 						URL:              url,
// 						Content:          c.Text,
// 						CrawledAt:        time.Now(),
// 						ContentEmbedding: c.Vector,
// 					})
// 				}
// 			}(chunk))
// 		}

// 		if err := g.Wait(); err != nil {
// 			log.Printf("error inserting vector: %v", err)
// 		}

// 	})
// }
