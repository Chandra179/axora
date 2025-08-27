package crawler

import (
	"context"
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
		colly.MaxDepth(5),
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
		isRelevant, score, err := w.relevanceFilter.IsURLRelevant(content)
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
