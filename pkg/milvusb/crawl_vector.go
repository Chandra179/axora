package milvusdb

import (
	"axora/repository"
	"context"
	"fmt"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

const (
	CrawlCollectionName = "crawl_collection"
)

type CrawlClient struct {
	Client client.Client
}

func NewCrawlClient(client client.Client) *CrawlClient {
	return &CrawlClient{Client: client}
}

func (c *CrawlClient) CreateCrawlCollection(ctx context.Context) error {
	schema := &entity.Schema{
		CollectionName: CrawlCollectionName,
		Description:    "Example collection for vector search",
		Fields: []*entity.Field{
			{
				Name:       "id",
				DataType:   entity.FieldTypeInt64,
				PrimaryKey: true,
				AutoID:     true,
			},
			{
				Name:     "content",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					entity.TypeParamMaxLength: "512",
				},
			},
			{
				Name:     "content_embedding",
				DataType: entity.FieldTypeFloatVector,
				TypeParams: map[string]string{
					entity.TypeParamDim: "384", // 384-dimensional vectors
				},
			},
		},
	}

	err := c.Client.CreateCollection(ctx, schema, entity.DefaultShardNumber)
	if err != nil {
		return err
	}

	// Load collection into memory for searching
	err = c.Client.LoadCollection(ctx, CrawlCollectionName, false)
	if err != nil {
		return fmt.Errorf("failed to load collection: %w", err)
	}

	return nil
}

func (c *CrawlClient) InsertOne(ctx context.Context, doc *repository.CrawlVectorDoc) error {
	contentColumn := entity.NewColumnVarChar("content", []string{doc.Content})
	doc.ContentEmbedding = [][]float32{
		{1.1, 2.2, 3.3},
		{4.4, 5.5, 6.6},
	}
	embeddingColumn := entity.NewColumnFloatVector("content_embedding", 384, doc.ContentEmbedding)

	_, err := c.Client.Insert(ctx, CrawlCollectionName, "", contentColumn, embeddingColumn)
	if err != nil {
		return fmt.Errorf("failed to insert document: %w", err)
	}

	return nil
}
