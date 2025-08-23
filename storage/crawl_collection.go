package storage

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type CrawlRepository interface {
	InsertOne(ctx context.Context, doc *Doc) (*mongo.InsertOneResult, error)
	GetOne(ctx context.Context, url string) (*Doc, error)
}

type CrawlCollection struct {
	docCollection *mongo.Collection
}

type Doc struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	URL        string             `bson:"url"`
	Content    string             `bson:"content"`
	CrawledAt  time.Time          `bson:"crawled_at"`
	Statuscode int                `bson:"status_code"`
}

func NewCrawlCollection(db *mongo.Database) *CrawlCollection {
	return &CrawlCollection{
		docCollection: db.Collection("crawled_documents"),
	}
}

func (c *CrawlCollection) InsertOne(ctx context.Context, doc *Doc) (*mongo.InsertOneResult, error) {
	doc.CrawledAt = time.Now()
	return c.docCollection.InsertOne(ctx, doc)
}

func (c *CrawlCollection) GetOne(ctx context.Context, url string) (*Doc, error) {
	var doc Doc
	filter := bson.M{"url": url}
	err := c.docCollection.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}
