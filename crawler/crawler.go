package crawler

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"axora/pkg/chunking"
	"axora/relevance"
	"axora/repository"

	"github.com/gocolly/colly/v2"
	"golang.org/x/sync/errgroup"
)

type Worker struct {
	collector       *colly.Collector
	chunker         chunking.ChunkingClient
	crawlVectorRepo repository.CrawlVectorRepo
	extractor       *ContentExtractor
	relevanceFilter relevance.RelevanceFilterClient
	visitedURL      map[string]int
	maxURLVisits    int
	mutex           sync.RWMutex
}

func NewWorker(crawlVectorRepo repository.CrawlVectorRepo, extractor *ContentExtractor, chunker chunking.ChunkingClient) *Worker {
	c := colly.NewCollector(
		colly.UserAgent("Axora-Crawler/1.0"),
		colly.MaxDepth(3),
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
		maxURLVisits:    3,
	}

	return worker
}

func (w *Worker) Crawl(ctx context.Context, relevanceFilter relevance.RelevanceFilterClient, urls []string) {
	// Configured on runtime
	w.relevanceFilter = relevanceFilter

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
		if u.Scheme != "http" && u.Scheme != "https" {
			return
		}
		if u.Host == "" {
			return
		}
		w.mutex.Lock()
		defer w.mutex.Unlock()
		currentVisits := w.visitedURL[absoluteURL]
		if currentVisits >= w.maxURLVisits {
			log.Printf("[VISIT_COUNTER] BLOCKING URL: %s - exceeded max visits %d", absoluteURL, w.maxURLVisits)
			return
		}
		e.Request.Visit(absoluteURL)
	})

	w.collector.OnScraped(func(r *colly.Response) {
		url := r.Request.URL.String()
		content, err := w.extractor.ExtractText(string(r.Body), r.Request.URL)
		if err != nil {
			log.Printf("err extracting text: %v", err)
			return
		}
		if content.IsBoilerplate {
			return
		}

		chunks, err := w.chunker.ChunkText(content.Text)
		if err != nil {
			log.Printf("Error chunking text for URL: %s Error: %v", url, err)
			return
		}

		g, ctx := errgroup.WithContext(ctx)
		g.SetLimit(2)

		for _, chunk := range chunks {
			g.Go(func(c chunking.ChunkOutput) func() error {
				return func() error {
					isRelevant, score, err := w.relevanceFilter.IsContentRelevant(c.Text)
					if err != nil {
						return fmt.Errorf("err checking relevance: %v", err)
					}
					if !isRelevant {
						return nil
					}
					log.Printf("URL: %s Score: %.3f", url, score)
					return w.crawlVectorRepo.InsertOne(ctx, &repository.CrawlVectorDoc{
						URL:              url,
						Content:          c.Text,
						CrawledAt:        time.Now(),
						ContentEmbedding: c.Vector,
					})
				}
			}(chunk))
		}

		if err := g.Wait(); err != nil {
			log.Printf("error inserting vector: %v", err)
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
