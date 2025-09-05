package repository

import (
	"context"
	"time"
)

type CrawlVectorRepo interface {
	InsertOne(ctx context.Context, doc *CrawlVectorDoc) error
}

type CrawlVectorDoc struct {
	URL              string    `json:"url"`
	Content          string    `json:"content"`
	ContentEmbedding []float32 `json:"content_embedding"`
	CrawledAt        time.Time `json:"crawledAt"`
}
