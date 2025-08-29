package repository

import (
	"context"
	"time"
)

type CrawlVectorRepo interface {
	InsertOne(ctx context.Context, className string, doc *CrawlVectorDoc) error
}

type CrawlVectorDoc struct {
	URL       string    `json:"url"`
	Content   string    `json:"content"`
	CrawledAt time.Time `json:"crawledAt"`
}
