package mongodb

import (
	"axora/repository"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type CrawlClient struct {
	col *mongo.Collection
}

func NewCrawlCollection(db *mongo.Database) *CrawlClient {
	col := db.Collection("crawled_documents")
	ctx := context.Background()
	indexModel := mongo.IndexModel{
		Keys: bson.D{{Key: "url", Value: 1}},
		Options: &options.IndexOptions{
			Unique: &[]bool{true}[0],
		},
	}
	col.Indexes().CreateOne(ctx, indexModel)
	return &CrawlClient{col: col}
}

func (c *CrawlClient) InsertOne(ctx context.Context, doc *repository.CrawlCollectionDoc) error {
	doc.CrawledAt = time.Now()
	_, err := c.col.InsertOne(ctx, doc)
	if err != nil {
		return fmt.Errorf("crawlcol: %w", err)
	}
	return nil
}

func (c *CrawlClient) GetOne(ctx context.Context, url string) (*repository.CrawlCollectionDoc, error) {
	var doc repository.CrawlCollectionDoc
	filter := bson.M{"url": url}
	err := c.col.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		return nil, fmt.Errorf("crawlcol: %w", err)
	}
	return &doc, nil
}
