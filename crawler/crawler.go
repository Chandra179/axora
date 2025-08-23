package crawler

import (
	"context"
	"log"
	"strings"
	"time"

	"axora/storage"

	"github.com/gocolly/colly/v2"
)

type Worker struct {
	collector        *colly.Collector
	crawlRepo        storage.CrawlRepository
	extractor        *ContentExtractor
	relevanceFilter  RelevanceFilter
}

func NewWorker(crawlRepo storage.CrawlRepository, extractor *ContentExtractor, keywords []string, relevanceThreshold float64) *Worker {
	c := colly.NewCollector(
		// colly.Debugger(&debug.LogDebugger{}),
		colly.UserAgent("Axora-Crawler/1.0"),
		colly.MaxDepth(1),
		// Enable async mode for better performance
		colly.Async(true),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		Delay:       1 * time.Second,
	})

	// Create relevance filter with provided keywords
	relevanceFilter, err := NewBleveRelevanceScorer(keywords, relevanceThreshold)
	if err != nil {
		log.Printf("Failed to create relevance filter: %v", err)
		relevanceFilter = nil
	}

	return &Worker{
		collector:       c,
		crawlRepo:       crawlRepo,
		extractor:       extractor,
		relevanceFilter: relevanceFilter,
	}
}

func (w *Worker) Crawl(ctx context.Context, urls []string) {
	w.collector.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		
		// Skip if no relevance filter is available
		if w.relevanceFilter == nil {
			w.collector.Visit(link)
			return
		}
		
		// Extract title and description for relevance check
		title := strings.TrimSpace(e.Text)
		
		// Try to get meta description from the current page context
		metaDescription := ""
		if metaDesc := e.DOM.Parents().Find("meta[name='description']").First(); metaDesc.Length() > 0 {
			metaDescription, _ = metaDesc.Attr("content")
		}
		
		// Check if the URL is relevant before visiting
		isRelevant, score, err := w.relevanceFilter.IsURLRelevant(link, title, metaDescription)
		if err != nil {
			log.Printf("Error checking relevance for %s: %v", link, err)
			return
		}
		
		if isRelevant {
			log.Printf("Following relevant link %s (score: %.3f)", link, score)
			w.collector.Visit(link)
		} else {
			log.Printf("Skipping irrelevant link %s (score: %.3f)", link, score)
		}
	})

	w.collector.OnScraped(func(r *colly.Response) {
		url := r.Request.URL.String()

		extractedText, _ := w.extractor.ExtractText(string(r.Body))
		
		// Double-check relevance with full content if filter is available
		if w.relevanceFilter != nil {
			isRelevant, score, err := w.relevanceFilter.IsURLRelevant(url, "", extractedText)
			if err != nil {
				log.Printf("Error checking content relevance for %s: %v", url, err)
				return
			}
			
			if !isRelevant {
				log.Printf("Skipping irrelevant content from %s (score: %.3f)", url, score)
				return
			}
			
			log.Printf("Saving relevant content from %s (score: %.3f)", url, score)
		}
		
		crawlData := &storage.Doc{
			URL:        url,
			Content:    extractedText,
			Statuscode: r.StatusCode,
			CrawledAt:  time.Now(),
		}

		_, err := w.crawlRepo.InsertOne(ctx, crawlData)
		if err != nil {
			log.Printf("Failed to save %s: %v", url, err)
		}
	})

	w.collector.OnResponse(func(r *colly.Response) {
		log.Printf("Visited %s (Status: %d)", r.Request.URL, r.StatusCode)
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
