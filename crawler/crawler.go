package crawler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"axora/storage"

	"github.com/gocolly/colly/v2"
)

type Worker struct {
	collector       *colly.Collector
	crawlRepo       storage.CrawlRepository
	extractor       *ContentExtractor
	relevanceFilter RelevanceFilter
}

func NewWorker(crawlRepo storage.CrawlRepository, extractor *ContentExtractor, relevanceFilter RelevanceFilter) *Worker {
	c := colly.NewCollector(
		// colly.Debugger(&debug.LogDebugger{}),
		colly.UserAgent("Axora-Crawler/1.0"),
		colly.MaxDepth(2),
		// Enable async mode for better performance
		colly.Async(true),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		Delay:       1 * time.Second,
	})

	return &Worker{
		collector:       c,
		crawlRepo:       crawlRepo,
		extractor:       extractor,
		relevanceFilter: relevanceFilter,
	}
}

func (w *Worker) Crawl(ctx context.Context, urls []string) {
	// Store page meta description when page loads
	w.collector.OnHTML("meta[name='description']", func(e *colly.HTMLElement) {
		content := e.Attr("content")
		if content != "" {
			e.Request.Ctx.Put("page_meta_description", content)
			fmt.Printf("Stored meta description: %s\n", content)
		}
	})

	// Extract and store headings for semantic analysis
	w.collector.OnHTML("h1, h2, h3", func(e *colly.HTMLElement) {
		headingText := strings.TrimSpace(e.Text)
		if headingText != "" {
			existing := ""
			if h := e.Request.Ctx.GetAny("page_headings"); h != nil {
				if existingStr, ok := h.(string); ok {
					existing = existingStr
				}
			}
			if existing != "" {
				existing += " " + headingText
			} else {
				existing = headingText
			}
			e.Request.Ctx.Put("page_headings", existing)
		}
	})

	w.collector.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		title := strings.TrimSpace(e.Text)
		timestamp := time.Now().Format("2006-01-02 15:04:05")

		// Get meta description from the current page context
		metaDescription := ""
		if metaDesc := e.Request.Ctx.GetAny("page_meta_description"); metaDesc != nil {
			if desc, ok := metaDesc.(string); ok {
				metaDescription = desc
			}
		}

		// Check if the URL is relevant before visiting
		isRelevant, score, err := w.relevanceFilter.IsURLRelevant(title, metaDescription)
		if err != nil {
			log.Printf("[%s] Error checking relevance for URL: %s, Error: %v", timestamp, link, err)
			return
		}

		if isRelevant {
			log.Printf("[%s] Following relevant link - URL: %s, Title: '%s', Meta: '%s', Score: %.3f",
				timestamp, link, title, metaDescription, score)
			w.collector.Visit(link)
		} else {
			log.Printf("[%s] Skipping irrelevant link - URL: %s, Title: '%s', Meta: '%s', Score: %.3f",
				timestamp, link, title, metaDescription, score)
		}
	})

	w.collector.OnScraped(func(r *colly.Response) {
		url := r.Request.URL.String()
		extractedText, _ := w.extractor.ExtractText(string(r.Body), r.Request.URL)
		timestamp := time.Now()

		crawlData := &storage.Doc{
			URL:        url,
			Content:    extractedText,
			Statuscode: r.StatusCode,
			CrawledAt:  timestamp,
		}

		log.Printf("[%s] Scraped content - URL: %s, Status: %d, Content length: %d chars, Content preview: '%.200s...'",
			timestamp.Format("2006-01-02 15:04:05"), url, r.StatusCode, len(extractedText), extractedText)

		_, err := w.crawlRepo.InsertOne(ctx, crawlData)
		if err != nil {
			log.Printf("[%s] Failed to save URL: %s, Error: %v", timestamp.Format("2006-01-02 15:04:05"), url, err)
		} else {
			log.Printf("[%s] Successfully saved URL: %s to database", timestamp.Format("2006-01-02 15:04:05"), url)
		}
	})

	w.collector.OnResponse(func(r *colly.Response) {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		log.Printf("[%s] HTTP Response - URL: %s, Status: %d, Content-Length: %d bytes",
			timestamp, r.Request.URL, r.StatusCode, len(r.Body))
	})

	for _, url := range urls {
		err := w.collector.Visit(url)
		if err != nil {
			log.Printf("Failed to visit %s: %v", url, err)
		}
	}

	w.collector.Wait()
}

// Close cleans up resources used by the worker
func (w *Worker) Close() error {
	if w.relevanceFilter != nil {
		return w.relevanceFilter.Close()
	}
	return nil
}
