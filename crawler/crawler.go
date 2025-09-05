package crawler

import (
	"context"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"axora/embedding"
	"axora/repository"

	"github.com/gocolly/colly/v2"
	"github.com/tmc/langchaingo/textsplitter"
)

type Worker struct {
	collector       *colly.Collector
	crawlVectorRepo repository.CrawlVectorRepo
	extractor       *ContentExtractor
	relevanceFilter RelevanceFilter
	embeddingClient embedding.Client
	loopDetector    *LoopDetector
}

func NewWorker(crawlVectorRepo repository.CrawlVectorRepo, embeddingClient embedding.Client, extractor *ContentExtractor) *Worker {
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
		crawlVectorRepo: crawlVectorRepo,
		extractor:       extractor,
		loopDetector:    loopDetector,
		embeddingClient: embeddingClient,
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

		splitter := textsplitter.NewRecursiveCharacter(
			textsplitter.WithChunkSize(50),
			textsplitter.WithChunkOverlap(10),
		)
		chunks, err := splitter.SplitText(content)
		if err != nil {
			log.Printf("splitter: %v", err)
			return
		}

		maxConcurrent := 3
		sem := make(chan struct{}, maxConcurrent)
		var wg sync.WaitGroup

		for _, chunk := range chunks {
			wg.Add(1)

			go func(c string) {
				defer wg.Done()

				// acquire slot
				sem <- struct{}{}
				defer func() { <-sem }() // release slot when done

				contentEmbed, err := w.embeddingClient.GetEmbeddings(ctx, []string{c})
				if err != nil {
					log.Printf("Embed process failed: %s", err)
					return
				}
				err = w.crawlVectorRepo.InsertOne(ctx, &repository.CrawlVectorDoc{
					URL:              url,
					Content:          chunk,
					CrawledAt:        timestamp,
					ContentEmbedding: contentEmbed[0],
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
