package crawler

import (
	"regexp"
	"sort"
	"strings"

	"axora/search"
)

type KeywordScore struct {
	Keyword string
	Score   float64
}

type RAKEExtractor struct {
	stopWords    map[string]bool
	punctuation  *regexp.Regexp
	wordSeparator *regexp.Regexp
}

func NewRAKEExtractor() *RAKEExtractor {
	stopWords := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
		"be": true, "been": true, "by": true, "for": true, "from": true, "has": true,
		"he": true, "in": true, "is": true, "it": true, "its": true, "of": true,
		"on": true, "that": true, "the": true, "to": true, "was": true, "will": true,
		"with": true, "would": true, "could": true, "should": true, "may": true,
		"might": true, "can": true, "must": true, "shall": true, "this": true,
		"these": true, "they": true, "them": true, "their": true, "there": true,
		"then": true, "than": true, "or": true, "but": true, "not": true, "no": true,
		"nor": true, "so": true, "yet": true, "however": true, "therefore": true,
		"thus": true, "hence": true, "because": true, "since": true, "although": true,
		"though": true, "unless": true, "until": true, "while": true, "where": true,
		"when": true, "who": true, "whom": true, "whose": true, "which": true,
		"what": true, "why": true, "how": true, "if": true, "do": true, "does": true,
		"did": true, "have": true, "had": true, "having": true, "get": true, "got": true,
		"getting": true, "go": true, "going": true, "gone": true, "went": true,
		"come": true, "came": true, "coming": true, "take": true, "took": true,
		"taken": true, "taking": true, "make": true, "made": true, "making": true,
		"see": true, "saw": true, "seen": true, "seeing": true, "know": true,
		"knew": true, "known": true, "knowing": true, "say": true, "said": true,
		"saying": true, "think": true, "thought": true, "thinking": true,
	}

	return &RAKEExtractor{
		stopWords:     stopWords,
		punctuation:   regexp.MustCompile(`[^\w\s]`),
		wordSeparator: regexp.MustCompile(`\s+`),
	}
}

func (r *RAKEExtractor) extractCandidatePhrases(text string) []string {
	text = strings.ToLower(text)
	text = r.punctuation.ReplaceAllString(text, " ")
	text = r.wordSeparator.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	words := strings.Fields(text)
	
	var phrases []string
	var currentPhrase []string

	for _, word := range words {
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}

		if r.stopWords[word] {
			if len(currentPhrase) > 0 {
				phrases = append(phrases, strings.Join(currentPhrase, " "))
				currentPhrase = nil
			}
		} else {
			if len(word) >= 2 {
				currentPhrase = append(currentPhrase, word)
			}
		}
	}

	if len(currentPhrase) > 0 {
		phrases = append(phrases, strings.Join(currentPhrase, " "))
	}

	return phrases
}

func (r *RAKEExtractor) calculateWordScores(phrases []string) map[string]float64 {
	wordFreq := make(map[string]int)
	wordDegree := make(map[string]int)

	for _, phrase := range phrases {
		words := strings.Fields(phrase)
		phraseLength := len(words)
		
		for _, word := range words {
			wordFreq[word]++
			wordDegree[word] += phraseLength - 1
		}
	}

	wordScores := make(map[string]float64)
	for word, freq := range wordFreq {
		degree := wordDegree[word]
		wordScores[word] = float64(degree+freq) / float64(freq)
	}

	return wordScores
}

func (r *RAKEExtractor) scoreKeywordPhrases(phrases []string, wordScores map[string]float64) []KeywordScore {
	var keywordScores []KeywordScore

	for _, phrase := range phrases {
		words := strings.Fields(phrase)
		var phraseScore float64
		
		for _, word := range words {
			if score, exists := wordScores[word]; exists {
				phraseScore += score
			}
		}

		if phraseScore > 0 {
			keywordScores = append(keywordScores, KeywordScore{
				Keyword: phrase,
				Score:   phraseScore,
			})
		}
	}

	sort.Slice(keywordScores, func(i, j int) bool {
		return keywordScores[i].Score > keywordScores[j].Score
	})

	return keywordScores
}

func (r *RAKEExtractor) ExtractKeywords(text string, topK int) []string {
	phrases := r.extractCandidatePhrases(text)
	if len(phrases) == 0 {
		return nil
	}

	wordScores := r.calculateWordScores(phrases)
	keywordScores := r.scoreKeywordPhrases(phrases, wordScores)

	var keywords []string
	limit := topK
	if len(keywordScores) < limit {
		limit = len(keywordScores)
	}

	for i := 0; i < limit; i++ {
		keywords = append(keywords, keywordScores[i].Keyword)
	}

	return keywords
}

func ExtractKeywordsFromSearchResults(query string, results []search.SearchResult, topK int) []string {
	rake := NewRAKEExtractor()
	
	// Combine search query with search result titles and descriptions
	var combinedText strings.Builder
	combinedText.WriteString(query)
	
	for _, result := range results {
		if result.Title != "" {
			combinedText.WriteString(" ")
			combinedText.WriteString(result.Title)
		}
		if result.Description != "" {
			combinedText.WriteString(" ")
			combinedText.WriteString(result.Description)
		}
	}
	
	return rake.ExtractKeywords(combinedText.String(), topK)
}