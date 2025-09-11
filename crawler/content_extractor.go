package crawler

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/go-shiori/go-readability"
)

type ContentExtractor struct {
	MinWordCount   int
	MaxLinkDensity float64
	MaxListDensity float64
	JunkPatterns   []string
}

type ExtractionResult struct {
	Text          string
	IsBoilerplate bool
	Reason        string
	WordCount     int
}

func NewContentExtractor() *ContentExtractor {
	return &ContentExtractor{
		MinWordCount:   120,
		MaxLinkDensity: 30.0,
		MaxListDensity: 50.0,
		JunkPatterns: []string{
			`\b(home|about|contact|menu|navigation|subscribe|login|register|sign up)\b`,
			`\b(next page|previous|see more|load more|read more|continue reading)\b`,
			`\b(published on|author:|tags:|category:|share this|follow us)\b`,
			`\b(privacy policy|terms of service|copyright|all rights reserved)\b`,
			`\b(add to cart|purchase|buy now|checkout|payment|try free)\b`,
		},
	}
}

func (ce *ContentExtractor) ExtractText(htmlContent string, url *url.URL) (*ExtractionResult, error) {
	article, err := readability.FromReader(strings.NewReader(htmlContent), url)
	if err != nil {
		return nil, fmt.Errorf("readability error: %v", err)
	}

	extractedText := strings.TrimSpace(article.TextContent)
	lowerText := strings.ToLower(extractedText)

	result := &ExtractionResult{
		Text:      lowerText,
		WordCount: ce.countWords(lowerText),
	}

	if reason := ce.detectBoilerplate(lowerText); reason != "" {
		result.IsBoilerplate = true
		result.Reason = reason
	}

	return result, nil
}

func (ce *ContentExtractor) detectBoilerplate(extractedText string) string {
	if ce.countWords(extractedText) < ce.MinWordCount {
		return "too few words"
	}

	if ce.countSentences(extractedText) < 3 {
		return "too few sentences"
	}

	matchCount := 0
	for _, pattern := range ce.JunkPatterns {
		if matched, _ := regexp.MatchString(pattern, extractedText); matched {
			matchCount++
		}
	}
	if matchCount >= 3 {
		return "matches junk patterns"
	}

	return ""
}

func (ce *ContentExtractor) countWords(text string) int {
	words := strings.FieldsFunc(text, func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	})
	return len(words)
}

func (ce *ContentExtractor) countSentences(text string) int {
	sentences := regexp.MustCompile(`[.!?]+`).Split(text, -1)
	count := 0
	for _, sentence := range sentences {
		if len(strings.TrimSpace(sentence)) > 10 {
			count++
		}
	}
	return count
}

// ExtractContent is a convenience function that creates a ContentExtractor and extracts content in one call
func ExtractContent(htmlContent string, url *url.URL) (*ExtractionResult, error) {
	extractor := NewContentExtractor()
	return extractor.ExtractText(htmlContent, url)
}
