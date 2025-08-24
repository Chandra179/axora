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
		MaxPages: 2,
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
	// Keyword extraction
	// ==========
	keywordExtractor := search.NewSimpleKeywordExtractor() // Extract up to 10 keywords
	if err != nil {
		log.Printf("Failed to create keyword extractor: %v", err)
	}

	var keywords []string
	keywords, err = keywordExtractor.ExtractKeywords(searchReq.Query)
	if err != nil {
		log.Printf("Failed to extract keywords: %v", err)
		keywords = []string{searchReq.Query} // Fallback to using the query itself
	}

	log.Printf("Extracted keywords from '%s': %v", searchReq.Query, keywords)

	// ==========
	// Relevance filter
	// ==========
	relevanceThreshold := 0.1 // Adjust threshold as needed (0.0 = very permissive, 1.0 = very strict)
	relevanceFilter, err := crawler.NewBleveRelevanceScorer(keywords, relevanceThreshold)
	if err != nil {
		log.Printf("Failed to create relevance filter: %v", err)
		relevanceFilter = nil
	}

	worker := crawler.NewWorker(crawlCollection, extractor, relevanceFilter)
	defer worker.Close()

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
