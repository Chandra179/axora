package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"axora/client"
	"axora/config"
	"axora/crawler"
	milvusdbClient "axora/pkg/milvusb"
	mongodbClient "axora/pkg/mongodb"
	"axora/search"
)

func main() {
	fmt.Println("Start")
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// ==========
	// MONGO DATABASE
	// ==========
	mdb, err := mongodbClient.NewClient(cfg.MongoURL, cfg.MongoDatabaseName)
	if err != nil {
		log.Fatalf("Failed to initialize MongoDB: %v", err)
	}
	crawlCollection := mongodbClient.NewCrawlCollection(mdb)

	// ==========
	// MILVUS DATABASE
	// ==========
	wdb, err := milvusdbClient.NewClient(cfg.MilvusPort, cfg.MilvusPort)
	if err != nil {
		log.Fatalf("Failed to initialize Weaviate: %v", err)
	}
	crawlVector := milvusdbClient.NewCrawlClient(wdb)
	crawlVector.CreateCrawlCollection(context.Background())

	// ==========
	// EXTRACTOR
	// ==========
	extractor := crawler.NewContentExtractor()

	// ==========
	// TEI MODEL CLIENT
	// ==========
	teiClient := client.NewTEIClient(cfg.TEIModelClientURL)

	// ==========
	// Relevance filter
	// ==========
	semanticRelevance, err := crawler.NewSemanticRelevanceFilter(teiClient, 0.61)
	if err != nil {
		log.Fatalf("Failed to initialize semantic relevance filter: %v", err)
	}

	// ==========
	// Crawler worker
	// ==========
	worker := crawler.NewWorker(crawlCollection, crawlVector, extractor)

	// ==========
	// Search
	// ==========
	serp := search.NewSerpApiSearchEngine(cfg.SerpApiKey)

	// ==========
	// HTTP
	// ==========
	http.Handle("/search", Crawl(serp, worker, teiClient, semanticRelevance))

	fmt.Println("Running")
	log.Fatal(http.ListenAndServe(":8000", nil))
}

func Crawl(serp *search.SerpApiSearchEngine, worker *crawler.Worker,
	teiClient *client.TEIClient, sem *crawler.SemanticRelevanceFilter) http.HandlerFunc {
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

		var filter crawler.RelevanceFilter
		if body.CrawlType == "semantic" {
			ctx := context.Background()
			embeddings, err := teiClient.GetEmbeddings(ctx, []string{body.Query})
			if err != nil {
				http.Error(w, "error tei model", http.StatusInternalServerError)
			}
			sem.QueryEmbedding = embeddings[0]
			filter = sem
		} else {
			rf, err := crawler.NewKeywordRelevanceFilter(body.Query)
			if err != nil {
				http.Error(w, "error keyword relevancne filter", http.StatusInternalServerError)
			}
			filter = rf
		}
		worker.Crawl(context.Background(), filter, []string{"https://news.ycombinator.com/"})
	}
}
