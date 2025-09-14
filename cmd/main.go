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

	// semanticRelevance, err := relevance.NewSemanticRelevanceFilter(mpnetbasev2, 0.61)
	// if err != nil {
	// 	log.Fatalf("Failed to initialize semantic relevance filter: %v", err)
	// }

	// ==========
	// Crawler worker
	// ==========
	worker := crawler.NewWorker(qdb, extractor, recurCharChunking)

	// ==========
	// HTTP
	// ==========
	http.Handle("/search", Crawl(search, worker, mpnetbasev2))

	fmt.Println("Running")
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(cfg.AppPort), nil))
}

func Crawl(serp *search.SerpApiSearchEngine, worker *crawler.Worker, embed embedding.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type RequestBody struct {
			Query string `json:"query"`
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

		worker.Crawl(context.Background(), []string{"https://libgen.li/ads.php?md5=5e4a98758351903d7412aa5c8cb3aa04"})
	}
}
