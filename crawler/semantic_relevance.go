package crawler

import (
	"axora/client"
	"context"
	"fmt"
)

type SemanticRelevanceFilter struct {
	teiClient      client.TEIHandler
	QueryEmbedding []float64
	threshold      float64
}

func NewSemanticRelevanceFilter(teiClient client.TEIHandler, threshold float64) (*SemanticRelevanceFilter, error) {
	return &SemanticRelevanceFilter{
		teiClient: teiClient,
		threshold: threshold,
	}, nil
}

func (s *SemanticRelevanceFilter) IsURLRelevant(content string) (bool, float64, error) {
	if content == "" {
		return false, 0.0, nil
	}
	ctx := context.Background()
	tc := truncateText(content, 200)
	embeddings, err := s.teiClient.GetEmbeddings(ctx, []string{tc})
	if err != nil {
		return false, 0.0, fmt.Errorf("failed to get content embedding: %w", err)
	}
	contentEmbedding := embeddings[0]

	similarity := client.CosineSimilarity(s.QueryEmbedding, contentEmbedding)
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
