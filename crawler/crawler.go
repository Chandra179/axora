package crawler

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"axora/storage"

	"github.com/gocolly/colly/v2"
)

type LoopDetector struct {
	visitCounts map[string]int
	visitOrder  []string
	maxEntries  int
	maxVisits   int
	mutex       sync.RWMutex
}

func NewLoopDetector(maxEntries, maxVisits int) *LoopDetector {
	return &LoopDetector{
		visitCounts: make(map[string]int),
		visitOrder:  make([]string, 0),
		maxEntries:  maxEntries,
		maxVisits:   maxVisits,
	}
}

func (ld *LoopDetector) CheckLoop(url string) bool {
	ld.mutex.Lock()
	defer ld.mutex.Unlock()

	count := ld.visitCounts[url]
	if count >= ld.maxVisits {
		return true
	}

	if count == 0 {
		if len(ld.visitOrder) >= ld.maxEntries {
			oldest := ld.visitOrder[0]
			delete(ld.visitCounts, oldest)
			ld.visitOrder = ld.visitOrder[1:]
		}
		ld.visitOrder = append(ld.visitOrder, url)
	}

	ld.visitCounts[url]++
	return false
}

type Worker struct {
	collector       *colly.Collector
	crawlRepo       storage.CrawlRepository
	extractor       *ContentExtractor
	relevanceFilter RelevanceFilter
	loopDetector    *LoopDetector
}

func NewWorker(crawlRepo storage.CrawlRepository, extractor *ContentExtractor, relevanceFilter RelevanceFilter) *Worker {
	c := colly.NewCollector(
		colly.UserAgent("Axora-Crawler/1.0"),
		colly.MaxDepth(5),
		colly.Async(true),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		Delay:       1 * time.Second,
	})

	loopDetector := NewLoopDetector(10000, 3)

	worker := &Worker{
		collector:       c,
		crawlRepo:       crawlRepo,
		extractor:       extractor,
		relevanceFilter: relevanceFilter,
		loopDetector:    loopDetector,
	}

	worker.setupLoopDetection()

	return worker
}

func (w *Worker) setupLoopDetection() {
	w.collector.OnRequest(func(r *colly.Request) {
		if w.loopDetector.CheckLoop(r.URL.String()) {
			log.Printf("Loop detected for URL: %s - aborting request", r.URL.String())
			r.Abort()
		}
	})
}

func (w *Worker) Crawl(ctx context.Context, urls []string) {
	w.collector.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		text := strings.TrimSpace(e.Text)
		absoluteURL := e.Request.AbsoluteURL(link)

		doc := e.DOM.Parents().Last()
		title := doc.Find("title").Text()
		metaDescription, _ := doc.Find("meta[name='description']").Attr("content")

		context := strings.TrimSpace(text + " " + title + " " + metaDescription)
		isRelevant, score, err := w.relevanceFilter.IsURLRelevant(context)
		if err != nil {
			log.Printf("Error checking relevance for URL: %s, Error: %v", absoluteURL, err)
			return
		}
		if isRelevant {
			log.Printf("relevant link - URL: %s, Context: %s, Score: %.3f", absoluteURL, context, score)
			e.Request.Visit(absoluteURL)
			return
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

		_, err := w.crawlRepo.InsertOne(ctx, crawlData)
		if err != nil {
			log.Printf("[%s] Failed to save URL: %s, Error: %v", timestamp.Format("2006-01-02 15:04:05"), url, err)
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
