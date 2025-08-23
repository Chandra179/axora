package main

import (
	"context"
	"fmt"
	"log"
	"time"

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

	client, err := initMongoDB(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize MongoDB: %v", err)
	}

	db := client.Database(cfg.MongoDatabase)
	crawlCollection := storage.NewCrawlCollection(db)

	searchEngine := search.NewSerpApiSearchEngine(cfg.SerpApiKey)
	searchReq := &search.SearchRequest{
		Query:    "bitcoin price prediction",
		MaxPages: 2,
	}

	searchResults, err := searchEngine.Search(context.Background(), searchReq)
	if err != nil {
		log.Fatalf("Failed to search: %v", err)
	}

	urls := extractURLsFromSearchResults(searchResults)
	log.Printf("Found %d URLs to crawl", len(urls))

	// Extract keywords using RAKE from search query and results
	keywords := crawler.ExtractKeywordsFromSearchResults(searchReq.Query, searchResults, 10)
	log.Printf("Extracted keywords: %v", keywords)

	worker := crawler.NewWorker(crawlCollection, keywords)
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
