package crawler

import (
	"context"
	"fmt"
	"log"
	"net/url"
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
	loopDetector    *LoopDetector
}

func NewWorker(crawlRepo storage.CrawlRepository, extractor *ContentExtractor) *Worker {
	c := colly.NewCollector(
		colly.UserAgent("Axora-Crawler/1.0"),
		colly.MaxDepth(2),
		colly.Async(true),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 3,
	})

	loopDetector := NewLoopDetector(3)

	worker := &Worker{
		collector:    c,
		crawlRepo:    crawlRepo,
		extractor:    extractor,
		loopDetector: loopDetector,
	}

	return worker
}

// TruncateString approximates token length by character count.
// Safe upper bound: ~4 chars ≈ 1 token (English).
// So for 1024 tokens, use ~4000 chars.
func truncateText(text string, maxTokens int) string {
	// Simple approximation: ~4 chars per token for English
	maxChars := maxTokens * 4
	if len(text) <= maxChars {
		return text
	}
	return text[:maxChars]
}

func isVisitableURL(str string) bool {
	u, err := url.ParseRequestURI(str)
	if err != nil {
		return false // not even a valid URI
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if u.Host == "" {
		return false
	}
	return true
}

func (w *Worker) Crawl(ctx context.Context, relevanceFilter RelevanceFilter, urls []string) {
	// Configured on runtime
	w.relevanceFilter = relevanceFilter

	w.collector.OnRequest(func(r *colly.Request) {
		w.loopDetector.IncVisit(r.URL.String())
	})

	w.collector.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		absoluteURL := e.Request.AbsoluteURL(link)
		if !isVisitableURL(absoluteURL) {
			return
		}
		if w.loopDetector.CheckLoop(absoluteURL) {
			log.Printf("[LOOP_DETECTOR] BLOCKING URL: %s - exceeded max visits %d", absoluteURL, w.loopDetector.maxVisits)
			return
		}
		e.Request.Visit(absoluteURL)
	})

	w.collector.OnScraped(func(r *colly.Response) {
		url := r.Request.URL.String()
		content, _ := w.extractor.ExtractText(string(r.Body), r.Request.URL)
		timestamp := time.Now()
		if strings.TrimSpace(content) == "" {
			return
		}
		tc := truncateText(content, 200)
		fmt.Println("content: " + tc)
		isRelevant, score, err := w.relevanceFilter.IsURLRelevant(tc)
		if err != nil {
			log.Printf("Error checking relevance for URL: %s Error: %v", url, err)
			return
		}
		log.Printf("URL: %s Score: %.3f", url, score)
		if !isRelevant {
			return
		}

		crawlData := &storage.Doc{
			URL:        url,
			Content:    content,
			Statuscode: r.StatusCode,
			CrawledAt:  timestamp,
		}
		_, err = w.crawlRepo.InsertOne(ctx, crawlData)
		if err != nil {
			log.Printf("[%s] Failed to save URL: %s Error: %v", timestamp.Format("2006-01-02 15:04:05"), url, err)
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
