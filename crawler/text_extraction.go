package crawler

import (
	"bytes"
	"net/url"
	"regexp"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/go-shiori/go-readability"
	"github.com/markusmobius/go-trafilatura"
	"go.uber.org/zap"
	"golang.org/x/net/html"
)

type Content struct {
	HtmlNode    string
	TextContent string
	TextMd      string
	Metadata    *ContentMetadata
}

type ContentMetadata struct {
	Title         string
	Author        string
	Description   string
	SiteName      string
	PublishedDate *time.Time
	ModifiedDate  *time.Time
	Language      string
	Tags          []string
	Categories    []string
	ImageURL      string
	License       string
	ID            string
	Fingerprint   string
	Excerpt       string
	CommentsCount int
	RawMetadata   map[string]interface{}
}

func (w *Crawler) ExtractWithTrafilatura(body []byte, pageURL string) (*Content, error) {
	reader := bytes.NewReader(body)

	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		w.logger.Error("trafilatura: failed to parse URL", zap.Error(err))
		return nil, err
	}

	opts := trafilatura.Options{
		OriginalURL: parsedURL,
	}

	result, err := trafilatura.Extract(reader, opts)
	if err != nil {
		w.logger.Error("trafilatura: extraction failed", zap.Error(err))
		return nil, err
	}
	htmlStr, err := RenderNodeToString(result.ContentNode)
	if err != nil {
		return nil, err
	}

	metadata := &ContentMetadata{
		Title:       result.Metadata.Title,
		Author:      result.Metadata.Author,
		Description: result.Metadata.Description,
		SiteName:    result.Metadata.Sitename,
		Language:    result.Metadata.Language,
		Tags:        result.Metadata.Tags,
		Categories:  result.Metadata.Categories,
		ImageURL:    result.Metadata.Image,
		License:     result.Metadata.License,
		ID:          result.Metadata.ID,
		Fingerprint: result.Metadata.Fingerprint,
		RawMetadata: make(map[string]interface{}),
	}

	textContent := result.ContentText
	words := strings.Fields(textContent)
	wordCount := len(words)

	w.logger.Info("trafilatura_extraction_result",
		zap.String("url", pageURL),
		zap.String("title", metadata.Title),
		zap.String("author", metadata.Author),
		zap.String("language", metadata.Language),
		zap.Int("word_count", wordCount),
		zap.Int("text_length", len(textContent)),
		zap.String("text", textContent),
		zap.Strings("tags", metadata.Tags),
		zap.Strings("categories", metadata.Categories),
		zap.String("content_node", htmlStr),
	)

	return &Content{
		HtmlNode:    htmlStr,
		TextContent: textContent,
		Metadata:    metadata,
	}, nil
}

func (w *Crawler) ExtractWithReadability(body []byte, pageURL string) (string, error) {
	reader := bytes.NewReader(body)

	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		w.logger.Error("readability: failed to parse URL", zap.Error(err))
		return "", err
	}

	parser := readability.NewParser()
	article, err := parser.Parse(reader, parsedURL)
	if err != nil {
		w.logger.Error("readability: extraction failed", zap.Error(err))
		return "", err
	}

	textContent := article.TextContent
	words := strings.Fields(textContent)
	wordCount := len(words)

	w.logger.Info("readability_extraction_result",
		zap.String("url", pageURL),
		zap.String("title", article.Title),
		zap.String("byline", article.Byline),
		zap.String("excerpt", article.Excerpt),
		zap.Int("word_count", wordCount),
		zap.Int("text_length", len(textContent)),
		zap.String("text", textContent),
	)

	return textContent, nil
}

func (w *Crawler) ExtractText(body []byte, pageURL string) (*Content, error) {
	content, err := w.ExtractWithTrafilatura(body, pageURL)
	if err != nil {
		return nil, err
	}
	// readabilityText, readabilityErr := w.ExtractWithReadability(body, pageURL)

	words := strings.Fields(content.TextContent)
	wordCount := len(words)
	htmlSize := len(content.HtmlNode)
	textSize := len(content.TextContent)
	lengthScoreVal := lengthScore(wordCount)

	unique := make(map[string]struct{}, len(words))
	for _, w := range words {
		w = strings.ToLower(strings.Trim(w, ".,!?\"'():;[]{}"))
		if w != "" {
			unique[w] = struct{}{}
		}
	}
	vocabRichness := float64(len(unique)) / float64(len(words))
	richnessScoreVal := richnessScore(vocabRichness)

	re := regexp.MustCompile(`[.!?]+`)
	sentences := re.Split(content.TextContent, -1)
	sentenceCount := len(sentences)
	if sentenceCount == 0 {
		sentenceCount = 1 // avoid divide by zero
	}
	avgSentenceLength := float64(wordCount) / float64(sentenceCount)
	sentenceScoreVal := sentenceScore(sentenceCount, avgSentenceLength)

	finalScore := qualityScore(lengthScoreVal, richnessScoreVal, sentenceScoreVal)
	if finalScore < 67 {
		return nil, nil
	}

	w.logger.Info("article_quality_metrics",
		zap.String("url", pageURL),
		zap.Int("word_count", wordCount),
		zap.Float64("vocab_richness", vocabRichness),
		zap.Int("sentence_count", sentenceCount),
		zap.Float64("avg_sentence_length", avgSentenceLength),
		zap.Int("html_size", htmlSize),
		zap.Int("text_size", textSize),
		zap.Float64("score", finalScore),
	)

	textMd, err := htmltomarkdown.ConvertString(content.HtmlNode)
	if err != nil {
		return nil, err
	}
	content.TextMd = textMd
	w.logger.Info("text_md", zap.String("text", textMd))

	return content, nil
}

func lengthScore(wordCount int) float64 {
	switch {
	case wordCount < 200:
		return 0.0
	case wordCount > 10000:
		return 0.7
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
	return (0.50*length + 0.30*richness + 0.20*sentence) * 100
}

func RenderNodeToString(n *html.Node) (string, error) {
	var buf bytes.Buffer
	if err := html.Render(&buf, n); err != nil {
		return "", err
	}
	return buf.String(), nil
}
