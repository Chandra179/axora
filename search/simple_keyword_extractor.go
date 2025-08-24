package search

import (
	"regexp"
	"strings"
)

// SimpleKeywordExtractor implements KeywordExtractor using stop word removal and basic stemming
type SimpleKeywordExtractor struct {
	stopWords map[string]bool
	stemmer   *SimpleStemmer
}

// NewSimpleKeywordExtractor creates a new simple keyword extractor
func NewSimpleKeywordExtractor() *SimpleKeywordExtractor {
	stopWords := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
		"be": true, "by": true, "for": true, "from": true, "has": true, "he": true,
		"in": true, "is": true, "it": true, "its": true, "of": true, "on": true,
		"that": true, "the": true, "to": true, "was": true, "were": true, "will": true,
		"with": true, "would": true, "could": true, "should": true, "may": true,
		"might": true, "can": true, "must": true, "shall": true, "do": true,
		"does": true, "did": true, "have": true, "had": true, "this": true,
		"these": true, "they": true, "them": true, "their": true, "his": true,
		"her": true, "she": true, "we": true, "you": true, "your": true,
		"our": true, "us": true, "me": true, "my": true, "i": true,
	}

	return &SimpleKeywordExtractor{
		stopWords: stopWords,
		stemmer:   NewSimpleStemmer(),
	}
}

// ExtractKeywords extracts keywords from a query using stop word removal and stemming
func (ske *SimpleKeywordExtractor) ExtractKeywords(query string) ([]string, error) {
	// Convert to lowercase
	query = strings.ToLower(query)
	
	// Remove punctuation except numbers and basic chars
	reg := regexp.MustCompile(`[^\w\s]`)
	query = reg.ReplaceAllString(query, " ")
	
	// Split into words
	words := strings.Fields(query)
	
	var keywords []string
	seen := make(map[string]bool)
	
	for _, word := range words {
		// Skip empty words
		if len(word) < 2 {
			continue
		}
		
		// Skip stop words
		if ske.stopWords[word] {
			continue
		}
		
		// Apply stemming
		stemmed := ske.stemmer.Stem(word)
		
		// Avoid duplicates
		if !seen[stemmed] {
			keywords = append(keywords, stemmed)
			seen[stemmed] = true
		}
	}
	
	return keywords, nil
}

// SimpleStemmer provides basic stemming functionality
type SimpleStemmer struct {
	suffixes []string
}

// NewSimpleStemmer creates a new simple stemmer
func NewSimpleStemmer() *SimpleStemmer {
	suffixes := []string{
		"ing", "ed", "er", "est", "ly", "tion", "sion", "ness", "ment",
		"able", "ible", "ful", "less", "ous", "ive", "al", "ic", "ical",
		"s", "es", "ies", "y",
	}
	
	return &SimpleStemmer{
		suffixes: suffixes,
	}
}

// Stem applies basic suffix removal stemming
func (ss *SimpleStemmer) Stem(word string) string {
	if len(word) < 4 {
		return word
	}
	
	original := word
	
	// Apply suffix removal rules
	for _, suffix := range ss.suffixes {
		if strings.HasSuffix(word, suffix) {
			stem := word[:len(word)-len(suffix)]
			if len(stem) >= 3 {
				return stem
			}
		}
	}
	
	return original
}