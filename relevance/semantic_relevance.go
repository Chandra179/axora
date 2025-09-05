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

func (s *SemanticRelevanceFilter) IsURLRelevant(content string) (bool, float32, error) {
	if content == "" {
		return false, 0.0, nil
	}
	ctx := context.Background()
	tc := truncateText(content, 200)
	embeddings, err := s.embeddingClient.GetEmbeddings(ctx, []string{tc})
	if err != nil {
		return false, 0.0, fmt.Errorf("failed to get content embedding: %w", err)
	}
	contentEmbedding := embeddings[0]

	similarity := embedding.CosineSimilarity(s.QueryEmbedding, contentEmbedding)
	isRelevant := similarity >= s.threshold

	return isRelevant, similarity, nil
}

// TruncateString approximates token length by character count.
// Safe upper bound: ~4 chars ≈ 1 token (English).
// So for 1024 tokens, use ~4000 chars.
func truncateText(text string, maxTokens int) string {
	// Simple approximation: ~4 chars per token for English
	maxChars := maxTokens * 4
	if len(text) <= maxChars {
		return text
	}
	return text[:maxChars]
}
