package crawler

import (
	"context"
	"log"
	"time"

	"axora/storage"

	"github.com/gocolly/colly/v2"
)

type Worker struct {
	collector *colly.Collector
	crawlRepo storage.CrawlRepository
	extractor *ReadabilityExtractor
}

func NewWorker(crawlRepo storage.CrawlRepository) *Worker {
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
		collector: c,
		crawlRepo: crawlRepo,
		extractor: NewReadabilityExtractor(),
	}
}

func (w *Worker) Crawl(ctx context.Context, url string) {
	w.collector.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		w.collector.Visit(link)
	})

	w.collector.OnScraped(func(r *colly.Response) {
		url := r.Request.URL.String()

		extractedText, err := w.extractor.ExtractText(string(r.Body), url)
		if err != nil {
			log.Printf("Failed to extract content from %s: %v", url, err)
			extractedText = string(r.Body)
		}

		crawlData := &storage.Doc{
			URL:        url,
			Content:    extractedText,
			Statuscode: r.StatusCode,
			CrawledAt:  time.Now(),
		}

		_, err = w.crawlRepo.InsertOne(ctx, crawlData)
		if err != nil {
			log.Printf("Failed to save %s: %v", url, err)
		}
	})

	w.collector.OnResponse(func(r *colly.Response) {
		log.Printf("Visited %s (Status: %d)", r.Request.URL, r.StatusCode)
	})

	err := w.collector.Visit(url)
	if err != nil {
		// log
	}

	w.collector.Wait()
}
