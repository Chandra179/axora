package crawler

import (
	"context"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"axora/pkg/chunking"
	"axora/relevance"
	"axora/repository"

	"github.com/gocolly/colly/v2"
)

type Worker struct {
	collector       *colly.Collector
	chunker         chunking.ChunkingClient
	crawlVectorRepo repository.CrawlVectorRepo
	extractor       *ContentExtractor
	relevanceFilter relevance.RelevanceFilterClient
	loopDetector    *LoopDetector
}

func NewWorker(crawlVectorRepo repository.CrawlVectorRepo, extractor *ContentExtractor, chunker chunking.ChunkingClient) *Worker {
	c := colly.NewCollector(
		colly.UserAgent("Axora-Crawler/1.0"),
		colly.MaxDepth(2),
		colly.Async(true),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
	})

	loopDetector := NewLoopDetector(3)

	worker := &Worker{
		collector:       c,
		chunker:         chunker,
		crawlVectorRepo: crawlVectorRepo,
		extractor:       extractor,
		loopDetector:    loopDetector,
	}

	return worker
}

func isVisitableURL(str string) bool {
	u, err := url.ParseRequestURI(str)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if u.Host == "" {
		return false
	}
	return true
}

func (w *Worker) Crawl(ctx context.Context, relevanceFilter relevance.RelevanceFilterClient, urls []string) {
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

		chunks, err := w.chunker.ChunkText(content)
		if err != nil {
			log.Printf("Error chunking text for URL: %s Error: %v", url, err)
			return
		}

		maxConcurrent := 3
		sem := make(chan struct{}, maxConcurrent)
		var wg sync.WaitGroup

		for _, chunk := range chunks {
			wg.Add(1)

			go func(c chunking.ChunkOutput) {
				defer wg.Done()

				// acquire slot
				sem <- struct{}{}
				defer func() { <-sem }() // release slot when done

				if len(chunk.Vector) == 0 {

				}
				err = w.crawlVectorRepo.InsertOne(ctx, &repository.CrawlVectorDoc{
					URL:              url,
					Content:          chunk.Text,
					CrawledAt:        timestamp,
					ContentEmbedding: chunk.Vector,
				})
				if err != nil {
					log.Printf("err insert vector: %s", err)
				}
			}(chunk)
		}

		wg.Wait()

	})

	for _, url := range urls {
		err := w.collector.Visit(url)
		if err != nil {
			log.Printf("Failed to visit %s: %v", url, err)
		}
	}

	w.collector.Wait()
}
