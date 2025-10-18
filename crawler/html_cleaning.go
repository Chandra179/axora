package crawler

import (
	"bytes"
	"net/url"
	"regexp"
	"strings"

	"github.com/go-shiori/go-readability"
	"go.uber.org/zap"
)

func (w *Crawler) CleanHTML(body []byte, pageURL string) (*readability.Article, error) {
	reader := bytes.NewReader(body)

	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		w.logger.Error("failed to parse URL", zap.Error(err))
		return nil, err
	}

	parser := readability.NewParser()
	article, err := parser.Parse(reader, parsedURL)
	if err != nil {
		w.logger.Error("readability extraction failed", zap.Error(err))
		return nil, err
	}

	words := strings.Fields(article.TextContent)
	wordCount := len(words)

	unique := make(map[string]struct{}, len(words))
	for _, w := range words {
		w = strings.ToLower(strings.Trim(w, ".,!?\"'():;[]{}"))
		if w != "" {
			unique[w] = struct{}{}
		}
	}
	vocabRichness := float64(len(unique)) / float64(len(words))

	re := regexp.MustCompile(`[.!?]+`)
	sentences := re.Split(article.TextContent, -1)
	sentenceCount := len(sentences)
	if sentenceCount == 0 {
		sentenceCount = 1 // avoid divide by zero
	}
	avgSentenceLength := float64(wordCount) / float64(sentenceCount)

	lengthScoreVal := lengthScore(wordCount)
	richnessScoreVal := richnessScore(vocabRichness)
	sentenceScoreVal := sentenceScore(sentenceCount, avgSentenceLength)

	finalScore := qualityScore(lengthScoreVal, richnessScoreVal, sentenceScoreVal)

	w.logger.Info("article_quality_metrics",
		zap.String("url", pageURL),
		zap.Int("word_count", wordCount),
		zap.Float64("vocab_richness", vocabRichness),
		zap.Int("sentence_count", sentenceCount),
		zap.Float64("avg_sentence_length", avgSentenceLength),
		zap.Float64("score", finalScore),
	)

	return &article, nil
}

func lengthScore(wordCount int) float64 {
	switch {
	case wordCount < 200:
		return 0.0
	case wordCount > 10000:
		return 0.3 // penalize long but not too harsh
	default:
		return 1.0 // ideal range
	}
}

func richnessScore(vocabRichness float64) float64 {
	switch {
	case vocabRichness < 0.25:
		return 0.0
	case vocabRichness > 0.6:
		return 0.8
	default:
		return 1.0
	}
}

func sentenceScore(sentenceCount int, avgSentenceLength float64) float64 {
	if sentenceCount < 5 {
		return 0.0
	}
	if avgSentenceLength < 10 || avgSentenceLength > 30 {
		return 0.7
	}
	return 1.0
}

func qualityScore(length, richness, sentence float64) float64 {
	return (0.6*length + 0.2*richness + 0.2*sentence) * 100
}
