package crawler

import (
	"context"
	"fmt"
	"strings"

	"axora/client"

	"github.com/cloudflare/ahocorasick"
)

type RelevanceFilter interface {
	IsURLRelevant(content string) (bool, float64, error)
}

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
	embeddings, err := s.teiClient.GetEmbeddings(ctx, []string{content})
	if err != nil {
		return false, 0.0, fmt.Errorf("failed to get content embedding: %w", err)
	}
	contentEmbedding := embeddings[0]

	similarity := client.CosineSimilarity(s.QueryEmbedding, contentEmbedding)
	isRelevant := similarity >= s.threshold

	return isRelevant, similarity, nil
}

// KeywordRelevanceFilter filters content based on a list of keywords/phrases.
type KeywordRelevanceFilter struct {
	matcher  *ahocorasick.Matcher
	keywords []string
}

// NewKeywordFilter initializes the filter with the given keywords/phrases.
// Query is a comma-separated list of keywords/phrases.
func NewKeywordRelevanceFilter(query string) (*KeywordRelevanceFilter, error) {
	// Split query into keywords/phrases
	parts := strings.Split(query, ",")
	keywords := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			keywords = append(keywords, strings.ToLower(p))
		}
	}

	// Build Aho-Corasick matcher
	matcher := ahocorasick.NewStringMatcher(keywords)

	return &KeywordRelevanceFilter{
		matcher:  matcher,
		keywords: keywords,
	}, nil
}

// IsURLRelevant checks if at least one keyword/phrase is in the content.
// Returns true if at least one keyword matches, along with a score (fraction of keywords found).
func (f *KeywordRelevanceFilter) IsURLRelevant(content string) (bool, float64, error) {
	if content == "" {
		return false, 0.0, nil
	}

	contentLower := strings.ToLower(content)

	// Run Aho-Corasick matcher
	matches := f.matcher.Match([]byte(contentLower))
	if len(matches) == 0 {
		return false, 0.0, nil
	}

	// Count unique matches for scoring
	found := make(map[string]struct{})
	for _, idx := range matches {
		found[f.keywords[idx]] = struct{}{}
	}

	// Score: fraction of keywords found
	score := float64(len(found)) / float64(len(f.keywords))

	return true, score, nil
}
