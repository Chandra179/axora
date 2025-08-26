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
	// SEARCH
	// ==========
	serp := search.NewSerpApiSearchEngine(cfg.SerpApiKey)
	http.Handle("/search", client.SearchHandler(serp))

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

	worker.Crawl(context.Background(), []string{"https://news.ycombinator.com/"})
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
