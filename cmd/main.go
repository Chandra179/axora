package main

import (
	"context"
	"fmt"
	"log"
	"time"

	teiclient "axora/client"
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
	client, err := initMongoDB(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize MongoDB: %v", err)
	}

	db := client.Database(cfg.MongoDatabase)
	crawlCollection := storage.NewCrawlCollection(db)

	// ==========
	// SEARCH
	// ==========
	searchEngine := search.NewSerpApiSearchEngine(cfg.SerpApiKey)
	searchReq := &search.SearchRequest{
		Query:    "information retrieval for llm",
		MaxPages: 5,
	}

	searchResults, err := searchEngine.Search(context.Background(), searchReq)
	if err != nil {
		log.Fatalf("Failed to search: %v", err)
	}
	urls := extractURLsFromSearchResults(searchResults)

	// ==========
	// EXTRACTOR
	// ==========
	extractor := crawler.NewContentExtractor()

	// ==========
	// TEI MODEL CLIENT
	// ==========
	teiClient := teiclient.NewTEIClient(cfg.TEIModelClientURL)

	// ==========
	// Relevance filter
	// ==========
	relevanceFilter, err := crawler.NewSemanticRelevanceFilter(
		teiClient,
		searchReq.Query,
		0.7, // threshold for relevance
	)
	if err != nil {
		log.Fatalf("Failed to initialize semantic relevance filter: %v", err)
	}

	worker := crawler.NewWorker(crawlCollection, extractor, relevanceFilter)

	worker.Crawl(context.Background(), urls)
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

func extractURLsFromSearchResults(results []search.SearchResult) []string {
	urls := make([]string, 0, len(results))
	for _, result := range results {
		if result.URL != "" {
			urls = append(urls, result.URL)
		}
	}
	return urls
}
