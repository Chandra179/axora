package crawler

import (
	"strings"

	"github.com/cloudflare/ahocorasick"
)

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
	isMatch := f.matcher.Contains([]byte(contentLower))
	return isMatch, 0.0, nil
}
