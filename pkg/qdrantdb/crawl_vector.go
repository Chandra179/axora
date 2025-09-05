package qdrantdb

import (
	"axora/repository"
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
)

const (
	CrawlCollectionName = "crawl_collection"
)

func (c *CrawlClient) CreateCrawlCollection(ctx context.Context) error {
	exists, err := c.Client.CollectionExists(ctx, CrawlCollectionName)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	err = c.Client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: CrawlCollectionName,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     768, // Adjust based on your embedding dimension
			Distance: qdrant.Distance_Cosine,
		}),
	})
	if err != nil {
		return fmt.Errorf("err create crawl collection: %w", err)
	}
	return nil
}

func (c *CrawlClient) InsertOne(ctx context.Context, doc *repository.CrawlVectorDoc) error {
	md := map[string]any{
		"url":     doc.URL,
		"content": doc.Content,
	}
	point := &qdrant.PointStruct{
		Id:      qdrant.NewID(uuid.New().String()),
		Vectors: qdrant.NewVectorsDense(doc.ContentEmbedding),
		Payload: qdrant.NewValueMap(md),
	}

	_, err := c.Client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: CrawlCollectionName,
		Points:         []*qdrant.PointStruct{point},
	})

	return err
}
