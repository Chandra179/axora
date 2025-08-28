package weaviatedb

import (
	"axora/repository"
	"context"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/weaviate/weaviate-go-client/v5/weaviate"
	"github.com/weaviate/weaviate/entities/models"
)

type CrawlClient struct {
	client *weaviate.Client
}

func NewCrawlClient(client *weaviate.Client) *CrawlClient {
	return &CrawlClient{client: client}
}

func (vc *CrawlClient) CreateCrawlSchema(ctx context.Context, className string) error {
	exists, err := vc.client.Schema().ClassExistenceChecker().WithClassName(className).Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to check class existence: %w", err)
	}
	if exists {
		return nil
	}

	classObj := &models.Class{
		Class: className,
		Properties: []*models.Property{
			{
				DataType: []string{"text"},
				Name:     "url",
			},
			{
				DataType: []string{"text"},
				Name:     "content",
			},
			{
				DataType: []string{"date"},
				Name:     "crawledAt",
			},
		},
		Vectorizer: "text2vec-huggingface",
	}

	return vc.client.Schema().ClassCreator().
		WithClass(classObj).
		Do(ctx)
}

func (vc *CrawlClient) InsertOne(ctx context.Context, className string, doc *repository.CrawlVectorDoc) error {
	cleanedContent := vc.cleanHTML(doc.Content)

	dataSchema := map[string]interface{}{
		"url":        doc.URL,
		"content":    cleanedContent,
		"crawledAt":  doc.CrawledAt.Format(time.RFC3339),
		"statusCode": doc.StatusCode,
	}

	// This creates new objects each time (but that's correct/required)
	_, err := vc.client.Data().Creator().
		WithClassName(className).
		WithProperties(dataSchema).
		Do(ctx)

	if err != nil {
		return fmt.Errorf("failed to insert document: %w", err)
	}

	return nil
}

func (vc *CrawlClient) cleanHTML(htmlContent string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	cleaned := re.ReplaceAllString(htmlContent, " ")
	cleaned = html.UnescapeString(cleaned)
	cleaned = regexp.MustCompile(`\s+`).ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(cleaned)

	return cleaned
}
