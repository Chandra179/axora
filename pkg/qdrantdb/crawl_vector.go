package qdrantdb

import (
	"axora/repository"
	"context"
	"crypto/sha256"
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

	_, err = c.Client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
		CollectionName: CrawlCollectionName,
		FieldName:      "content_hash",
		FieldType:      qdrant.FieldType_FieldTypeKeyword.Enum(),
	})
	if err != nil {
		return fmt.Errorf("err create content_hash index: %w", err)
	}
	return nil
}

func (c *CrawlClient) InsertOne(ctx context.Context, doc *repository.CrawlVectorDoc) error {
	hash := sha256.Sum256([]byte(doc.Content))
	hashBytes := hash[:16]
	namespace := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	id := uuid.NewSHA1(namespace, hashBytes).String()

	resp, err := c.Client.Get(ctx, &qdrant.GetPoints{
		CollectionName: CrawlCollectionName,
		Ids:            []*qdrant.PointId{qdrant.NewID(id)},
	})
	if err != nil {
		return err
	}
	if len(resp) > 0 {
		return nil
	}

	md := map[string]any{
		"url":     doc.URL,
		"content": doc.Content,
	}
	point := &qdrant.PointStruct{
		Id:      qdrant.NewID(id),
		Vectors: qdrant.NewVectorsDense(doc.ContentEmbedding),
		Payload: qdrant.NewValueMap(md),
	}

	_, err = c.Client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: CrawlCollectionName,
		Points:         []*qdrant.PointStruct{point},
	})

	return err
}
