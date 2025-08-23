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
	collector *colly.Collector
	crawlRepo storage.CrawlRepository
	extractor *ReadabilityExtractor
	keywords  []string
}

func NewWorker(crawlRepo storage.CrawlRepository, keywords []string) *Worker {
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
		collector: c,
		crawlRepo: crawlRepo,
		extractor: NewReadabilityExtractor(),
		keywords:  keywords,
	}
}

func (w *Worker) Crawl(ctx context.Context, urls []string) {
	w.collector.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		// Only visit link if it contains relevant keywords
		if w.isRelevant(e.Text) {
			log.Printf("Following relevant link: %s (context: %.100s)", link, e.Text)
			w.collector.Visit(link)
		}
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

	for _, url := range urls {
		err := w.collector.Visit(url)
		if err != nil {
			log.Printf("Failed to visit %s: %v", url, err)
		}
	}

	w.collector.Wait()
}

func (w *Worker) isRelevant(context string) bool {
	if len(w.keywords) == 0 {
		return true // If no keywords specified, accept all links
	}

	context = strings.ToLower(context)

	for _, keyword := range w.keywords {
		if strings.Contains(context, strings.ToLower(keyword)) {
			return true
		}
	}

	return false
}
