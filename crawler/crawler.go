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
		colly.MaxDepth(1),
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
	w.collector.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")

		fmt.Println("Link: " + link)
		fmt.Println("Text: " + e.Text)
		title := strings.TrimSpace(e.Text)

		// Try to get meta description from the current page context
		metaDescription := ""
		if metaDesc := e.DOM.Parents().Find("meta[name='description']").First(); metaDesc.Length() > 0 {
			metaDescription, _ = metaDesc.Attr("content")
			fmt.Println("Meta: " + metaDescription)
		}

		// Check if the URL is relevant before visiting
		isRelevant, score, err := w.relevanceFilter.IsURLRelevant(title, metaDescription)
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
