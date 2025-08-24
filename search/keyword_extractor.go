package search

// KeywordExtractor defines the interface for extracting keywords from search queries
type KeywordExtractor interface {
	ExtractKeywords(query string) ([]string, error)
}
