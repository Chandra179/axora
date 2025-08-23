package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"axora/config"
	"axora/crawler"
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

	worker := crawler.NewWorker(crawlCollection)
	worker.Crawl(context.Background(), "https://news.ycombinator.com/")
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
