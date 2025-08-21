package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"axora/config"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Loaded configuration - MongoDB: %s:%d, App Port: %d",
		cfg.MongoDatabase, cfg.MongoPort, cfg.AppPort)

	client, err := initMongoDB(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize MongoDB: %v", err)
	}
	fmt.Println(client)

	log.Println("MongoDB initialized successfully")
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
