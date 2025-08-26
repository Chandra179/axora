package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"axora/client"
	"axora/config"
	"axora/crawler"
	"axora/search"
	"axora/storage"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// ==========
	// DATABASE
	// ==========
	mongo, err := initMongoDB(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize MongoDB: %v", err)
	}

	db := mongo.Database(cfg.MongoDatabase)
	crawlCollection := storage.NewCrawlCollection(db)

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
	relevanceFilter, err := crawler.NewSemanticRelevanceFilter(
		teiClient,
		// searchReq.Query,
		"software engineer",
		0.61,
	)
	if err != nil {
		log.Fatalf("Failed to initialize semantic relevance filter: %v", err)
	}
	worker := crawler.NewWorker(crawlCollection, extractor, relevanceFilter)

	// ==========
	// HTTP
	// ==========
	serp := search.NewSerpApiSearchEngine(cfg.SerpApiKey)
	http.Handle("/search", CrawlFromSearch(serp, worker))

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func initMongoDB(cfg *config.Config) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(cfg.MongoURL)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	if err = client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	return client, nil
}

func CrawlFromSearch(serp *search.SerpApiSearchEngine, worker *crawler.Worker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if query == "" {
			http.Error(w, "missing query parameter", http.StatusBadRequest)
			return
		}
		searchResults, _ := serp.Search(context.Background(), &search.SearchRequest{
			Query:    query,
			MaxPages: 2,
		})
		res := make([]string, len(searchResults))
		for i := 0; i < len(searchResults); i++ {
			res = append(res, searchResults[i].URL)
		}
		worker.Crawl(context.Background(), res)
	}
}
