package repository

import (
	"context"
	"time"
)

type CrawlCollectionRepo interface {
	InsertOne(ctx context.Context, doc *CrawlCollectionDoc) error
	GetOne(ctx context.Context, url string) (*CrawlCollectionDoc, error)
}

type CrawlCollectionDoc struct {
	ID         string    `bson:"id"`
	URL        string    `bson:"url"`
	Content    string    `bson:"content"`
	CrawledAt  time.Time `bson:"crawled_at"`
	Statuscode int       `bson:"status_code"`
}
