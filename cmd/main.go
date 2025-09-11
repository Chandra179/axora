package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"axora/config"
	"axora/crawler"
	"axora/pkg/chunking"
	"axora/pkg/embedding"
	qdrantClient "axora/pkg/qdrantdb"
	"axora/relevance"
	"axora/search"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// ==========
	// DATABASE
	// ==========
	qdb, err := qdrantClient.NewClient(cfg.QdrantHost, cfg.QdrantPort)
	if err != nil {
		log.Fatalf("Failed to initialize Weaviate: %v", err)
	}
	err = qdb.CreateCrawlCollection(context.Background())
	if err != nil {
		log.Fatalf("err: %v", err)
	}

	// ==========
	//
	// ==========
	extractor := crawler.NewContentExtractor()
	mpnetbasev2 := embedding.NewMpnetBaseV2(cfg.AllMinilmL6V2URL)
	// semanticChunking := chunking.NewSemanticChunking(cfg.ChunkingURL)
	search := search.NewSerpApiSearchEngine(cfg.SerpApiKey)
	recurCharChunking := chunking.NewRecursiveCharacterChunking(mpnetbasev2)

	// ==========
	// Relevance filter
	// ==========
	semanticRelevance, err := relevance.NewSemanticRelevanceFilter(mpnetbasev2, 0.61)
	if err != nil {
		log.Fatalf("Failed to initialize semantic relevance filter: %v", err)
	}

	// ==========
	// Crawler worker
	// ==========
	worker := crawler.NewWorker(qdb, extractor, recurCharChunking)

	// ==========
	// HTTP
	// ==========
	http.Handle("/search", Crawl(search, worker, mpnetbasev2, semanticRelevance))

	fmt.Println("Running")
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(cfg.AppPort), nil))
}

func Crawl(serp *search.SerpApiSearchEngine, worker *crawler.Worker,
	embed embedding.Client, sem *relevance.SemanticRelevanceFilter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type RequestBody struct {
			Query     string `json:"query"`
			CrawlType string `json:"crawl_type"`
		}

		var body RequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}

		if body.Query == "" {
			http.Error(w, "missing query field", http.StatusBadRequest)
			return
		}
		if body.CrawlType == "" {
			http.Error(w, "missing crawl_type field", http.StatusBadRequest)
			return
		}

		// ==========
		// Seed urls
		// ==========
		// searchResults, _ := serp.Search(context.Background(), &search.SearchRequest{
		// 	Query:    query,
		// 	MaxPages: 2,
		// })
		// urls := make([]string, len(searchResults))
		// for i := 0; i < len(searchResults); i++ {
		// 	urls = append(urls, searchResults[i].URL)
		// }

		var filter relevance.RelevanceFilterClient
		if body.CrawlType == "semantic" {
			ctx := context.Background()
			embeddings, err := embed.GetEmbeddings(ctx, []string{body.Query})
			if err != nil {
				http.Error(w, "error tei model", http.StatusInternalServerError)
			}
			sem.QueryEmbedding = embeddings[0]
			filter = sem
		} else {
			rf, err := relevance.NewKeywordRelevanceFilter(body.Query)
			if err != nil {
				http.Error(w, "error keyword relevancne filter", http.StatusInternalServerError)
			}
			filter = rf
		}
		worker.Crawl(context.Background(), filter, []string{
			"https://openai.com/news/", "https://machinelearningmastery.com/blog/", "https://aws.amazon.com/blogs/machine-learning/",
			"https://bair.berkeley.edu/blog/", "https://research.google/blog/", "https://deepmind.google/discover/blog/",
			"https://news.mit.edu/topic/artificial-intelligence2", "https://www.kdnuggets.com/",
		})
	}
}
