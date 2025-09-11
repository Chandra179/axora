package relevance

import (
	"axora/pkg/embedding"
	"context"
	"fmt"
)

type SemanticRelevanceFilter struct {
	embeddingClient embedding.Client
	QueryEmbedding  []float32
	threshold       float32
}

func NewSemanticRelevanceFilter(embeddingClient embedding.Client, threshold float32) (*SemanticRelevanceFilter, error) {
	return &SemanticRelevanceFilter{
		embeddingClient: embeddingClient,
		threshold:       threshold,
	}, nil
}

func (s *SemanticRelevanceFilter) IsContentRelevant(content string) (bool, float32, error) {
	if content == "" {
		return false, 0.0, nil
	}
	ctx := context.Background()
	embeddings, err := s.embeddingClient.GetEmbeddings(ctx, []string{content})
	if err != nil {
		return false, 0.0, fmt.Errorf("failed to get content embedding: %w", err)
	}
	contentEmbedding := embeddings[0]

	similarity := embedding.CosineSimilarity(s.QueryEmbedding, contentEmbedding)
	isRelevant := similarity >= s.threshold

	return isRelevant, similarity, nil
}
