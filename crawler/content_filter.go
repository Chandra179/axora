package crawler

import (
	"math"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/jdkato/prose/v2"
)

type ContentQualityConfig struct {
	MinTextLength    int     // Minimum text length in characters
	MinSentences     int     // Minimum number of sentences
	MinParagraphs    int     // Minimum number of paragraphs
	MinEntropy       float64 // Minimum Shannon entropy
	MinTTR           float64 // Minimum Type-Token Ratio
	MaxTextHTMLRatio float64 // Maximum ratio of HTML tags to text
	MinTextHTMLRatio float64 // Minimum ratio of text to HTML tags
}

func DefaultContentQualityConfig() ContentQualityConfig {
	return ContentQualityConfig{
		MinTextLength:    200,
		MinSentences:     2,
		MinParagraphs:    1,
		MinEntropy:       2.5, // Lower bound for natural language
		MinTTR:           0.3, // At least 30% unique tokens
		MaxTextHTMLRatio: 0.5, // No more than 50% HTML vs text
		MinTextHTMLRatio: 0.5, // At least 10% text content
	}
}

func IsContentRelevant(doc *goquery.Document) bool {
	return IsContentRelevantWithConfig(doc, DefaultContentQualityConfig())
}

func IsContentRelevantWithConfig(doc *goquery.Document, config ContentQualityConfig) bool {
	text := extractText(doc)

	if len(text) < config.MinTextLength {
		return false
	}

	paragraphCount := doc.Find("p").Length()
	if paragraphCount < config.MinParagraphs {
		return false
	}

	sentenceCount, entropy, ttr := analyzeTextQuality(text)

	if sentenceCount < config.MinSentences {
		return false
	}

	// Check entropy (vocabulary richness)
	if entropy < config.MinEntropy {
		return false
	}

	// Check Type-Token Ratio (vocabulary diversity)
	if ttr < config.MinTTR {
		return false
	}

	// Check text-to-HTML ratio
	textHTMLRatio := calculateTextHTMLRatio(doc, text)
	if textHTMLRatio > config.MaxTextHTMLRatio || textHTMLRatio < config.MinTextHTMLRatio {
		return false
	}

	return true
}

func extractText(doc *goquery.Document) string {
	// Remove script and style elements
	doc.Find("script, style, noscript").Each(func(i int, s *goquery.Selection) {
		s.Remove()
	})
	text := doc.Find("body").Text()
	text = strings.Join(strings.Fields(text), " ")

	return text
}

func analyzeTextQuality(text string) (sentenceCount int, entropy float64, ttr float64) {
	doc, err := prose.NewDocument(text)
	if err != nil {
		return 0, 0.0, 0.0
	}

	sentences := doc.Sentences()
	sentenceCount = len(sentences)

	// Tokenize for entropy and TTR calculation
	tokens := doc.Tokens()
	if len(tokens) == 0 {
		return sentenceCount, 0.0, 0.0
	}

	// Calculate token frequency for entropy
	tokenFreq := make(map[string]int)
	tokenSet := make(map[string]bool)

	for _, token := range tokens {
		word := strings.ToLower(token.Text)
		// Skip very short tokens and punctuation
		if len(word) > 1 && isAlphanumeric(word) {
			tokenFreq[word]++
			tokenSet[word] = true
		}
	}

	totalTokens := 0
	for _, count := range tokenFreq {
		totalTokens += count
	}

	// Calculate Shannon entropy
	entropy = 0.0
	if totalTokens > 0 {
		for _, count := range tokenFreq {
			probability := float64(count) / float64(totalTokens)
			entropy -= probability * math.Log2(probability)
		}
	}

	// Calculate Type-Token Ratio (unique tokens / total tokens)
	uniqueTokens := len(tokenSet)
	if totalTokens > 0 {
		ttr = float64(uniqueTokens) / float64(totalTokens)
	}

	return sentenceCount, entropy, ttr
}

// calculateTextHTMLRatio calculates the ratio of HTML tags to text content
func calculateTextHTMLRatio(doc *goquery.Document, text string) float64 {
	linkCount := doc.Find("a").Length()
	imgCount := doc.Find("img").Length()
	divCount := doc.Find("div").Length()
	spanCount := doc.Find("span").Length()

	// Total structural elements
	totalTags := linkCount + imgCount + divCount + spanCount

	if len(text) == 0 {
		return 1.0 // Maximum ratio (all HTML, no text)
	}

	// Calculate ratio: higher values mean more HTML structure relative to content
	ratio := float64(totalTags) / float64(len(text))

	return ratio
}

// isAlphanumeric checks if a string contains only alphanumeric characters
func isAlphanumeric(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}
