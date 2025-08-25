package crawler

import (
	"context"
	"fmt"

	"axora/client"
)

// RelevanceFilter defines the interface for URL relevance filtering
type RelevanceFilter interface {
	IsURLRelevant(source, target string) (bool, float64, error)
}

// SemanticRelevanceFilter implements semantic similarity-based relevance filtering using TEI
type SemanticRelevanceFilter struct {
	teiClient      client.TEIHandler
	queryEmbedding []float64
	threshold      float64
}

// NewSemanticRelevanceFilter creates a new semantic relevance filter
func NewSemanticRelevanceFilter(teiClient client.TEIHandler, query string, threshold float64) (*SemanticRelevanceFilter, error) {
	ctx := context.Background()
	embeddings, err := teiClient.GetEmbeddings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("failed to get query embedding: %w", err)
	}
	queryEmbedding := embeddings[0]

	return &SemanticRelevanceFilter{
		teiClient:      teiClient,
		queryEmbedding: queryEmbedding,
		threshold:      threshold,
	}, nil
}

// IsURLRelevant checks if a URL is relevant based on semantic similarity
func (s *SemanticRelevanceFilter) IsURLRelevant(title, description string) (bool, float64, error) {
	// Combine title and description for content analysis
	content := title
	if description != "" {
		content += " " + description
	}

	if content == "" {
		return false, 0.0, nil
	}

	ctx := context.Background()
	embeddings, err := s.teiClient.GetEmbeddings(ctx, []string{content})
	if err != nil {
		return false, 0.0, fmt.Errorf("failed to get content embedding: %w", err)
	}
	contentEmbedding := embeddings[0]

	similarity := client.CosineSimilarity(s.queryEmbedding, contentEmbedding)
	isRelevant := similarity >= s.threshold

	return isRelevant, similarity, nil
}
