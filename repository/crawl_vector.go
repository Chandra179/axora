package repository

import (
	"context"
	"time"
)

type CrawlVectorRepo interface {
	InsertOne(ctx context.Context, className string, doc *CrawlVectorDoc) error
}

type CrawlVectorDoc struct {
	ID         string    `json:"id,omitempty"`
	URL        string    `json:"url"`
	Content    string    `json:"content"`
	CrawledAt  time.Time `json:"crawledAt"`
	StatusCode int       `json:"statusCode"`
}
