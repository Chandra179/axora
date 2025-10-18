package crawler

import (
	"bytes"
	"regexp"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"github.com/markusmobius/go-trafilatura"
	"go.uber.org/zap"
)

func (w *Crawler) CleanHTML(body []byte) (*trafilatura.ExtractResult, *ContentMetrics, error) {
	reader := bytes.NewReader(body)
	result, err := trafilatura.Extract(reader, w.trafilaturaOpt)
	if err != nil {
		w.logger.Error("trafilatura extraction failed", zap.Error(err))
		return nil, nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		w.logger.Error("failed to parse HTML document", zap.Error(err))
		return nil, nil, err
	}

	metrics := w.analyzeContentQuality(body, result, doc)

	return result, metrics, nil
}

func (w *Crawler) analyzeContentQuality(htmlBody []byte, result *trafilatura.ExtractResult, doc *goquery.Document) *ContentMetrics {
	metrics := &ContentMetrics{
		HTMLLength:     len(htmlBody),
		FailureReasons: make([]string, 0),
	}

	if result == nil || result.ContentText == "" {
		metrics.FailureReasons = append(metrics.FailureReasons, "no content extracted")
		return metrics
	}

	text := result.ContentText
	metrics.Text = text
	metrics.metadata = result.Metadata

	words := w.extractWords(text)
	metrics.WordCount = len(words)
	metrics.TextLength = len(text)

	if metrics.HTMLLength > 0 {
		metrics.TextHTMLRatio = float64(metrics.TextLength) / float64(metrics.HTMLLength)
	}

	sentences := w.extractSentences(text)
	metrics.SentenceCount = len(sentences)
	if metrics.SentenceCount > 0 {
		metrics.AvgSentenceLength = float64(metrics.WordCount) / float64(metrics.SentenceCount)
	}

	uniqueWords := w.countUniqueWords(words)
	metrics.UniqueWords = uniqueWords
	if metrics.WordCount > 0 {
		metrics.VocabRichness = float64(uniqueWords) / float64(metrics.WordCount)
	}

	metrics.ParagraphCount, metrics.HeadingCount = w.analyzeStructuredContent(doc)
	metrics.HasParagraphs = metrics.ParagraphCount > 0
	metrics.HasHeadings = metrics.HeadingCount > 0

	metrics.ExternalLinkCount = w.countExternalLinks(doc)
	if metrics.WordCount > 0 {
		metrics.LinkDensity = float64(metrics.ExternalLinkCount) / float64(metrics.WordCount)
	}

	metrics.AdScriptCount = w.countAdScripts(doc)
	metrics.PassesQualityCheck = w.applyQualityRules(metrics)

	return metrics
}

func (w *Crawler) extractWords(text string) []string {
	text = strings.TrimSpace(text)
	wordRegex := regexp.MustCompile(`\b[\w'-]+\b`)
	words := wordRegex.FindAllString(text, -1)

	// Filter out very short words and numbers-only
	filtered := make([]string, 0, len(words))
	for _, word := range words {
		if len(word) > 1 && !isNumberOnly(word) {
			filtered = append(filtered, strings.ToLower(word))
		}
	}
	return filtered
}

func (w *Crawler) extractSentences(text string) []string {
	// Split by common sentence terminators
	sentenceRegex := regexp.MustCompile(`[.!?]+[\s\n]+`)
	sentences := sentenceRegex.Split(text, -1)

	// Filter out empty sentences
	filtered := make([]string, 0, len(sentences))
	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if len(sentence) > 10 { // Minimum sentence length
			filtered = append(filtered, sentence)
		}
	}
	return filtered
}

func (w *Crawler) countUniqueWords(words []string) int {
	uniqueMap := make(map[string]bool)
	for _, word := range words {
		uniqueMap[word] = true
	}
	return len(uniqueMap)
}

func (w *Crawler) analyzeStructuredContent(doc *goquery.Document) (paragraphs int, headings int) {
	paragraphs = doc.Find("p").Length()
	headings = doc.Find("h1, h2, h3, h4, h5, h6").Length()

	return paragraphs, headings
}

func (w *Crawler) countExternalLinks(doc *goquery.Document) int {
	externalCount := 0
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && (strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://")) {
			externalCount++
		}
	})

	return externalCount
}

func (w *Crawler) countAdScripts(doc *goquery.Document) int {
	adPatterns := []string{
		"doubleclick.net",
		"googlesyndication.com",
		"googleadservices.com",
		"google-analytics.com",
		"googletagmanager.com",
		"facebook.net",
		"connect.facebook.net",
		"scorecardresearch.com",
		"adnxs.com",
		"advertising.com",
		"criteo.com",
		"outbrain.com",
		"taboola.com",
		"amazon-adsystem.com",
		"adsafeprotected.com",
		"moatads.com",
		"/ads/",
		"/advertisement",
		"/tracking",
		"/analytics",
	}

	adCount := 0
	doc.Find("script[src]").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if exists {
			for _, pattern := range adPatterns {
				if strings.Contains(strings.ToLower(src), pattern) {
					adCount++
					break
				}
			}
		}
	})

	return adCount
}

func (w *Crawler) applyQualityRules(metrics *ContentMetrics) bool {
	rules := w.qualityRules
	passes := true

	// Check word count
	if metrics.WordCount < rules.MinWordCount {
		metrics.FailureReasons = append(metrics.FailureReasons,
			"word count too low")
		passes = false
	}

	// Check text-to-HTML ratio
	if metrics.TextHTMLRatio < rules.MinTextHTMLRatio {
		metrics.FailureReasons = append(metrics.FailureReasons,
			"text-to-HTML ratio too low (likely boilerplate)")
		passes = false
	}

	// Check sentence count
	if metrics.SentenceCount < rules.MinSentenceCount {
		metrics.FailureReasons = append(metrics.FailureReasons,
			"sentence count too low")
		passes = false
	}

	// Check average sentence length
	if metrics.AvgSentenceLength < float64(rules.MinAvgSentenceLength) {
		metrics.FailureReasons = append(metrics.FailureReasons,
			"average sentence length too short")
		passes = false
	}
	if metrics.AvgSentenceLength > float64(rules.MaxAvgSentenceLength) {
		metrics.FailureReasons = append(metrics.FailureReasons,
			"average sentence length too long")
		passes = false
	}

	// Check vocabulary richness
	if metrics.VocabRichness < rules.MinVocabRichness {
		metrics.FailureReasons = append(metrics.FailureReasons,
			"vocabulary richness too low (repetitive content)")
		passes = false
	}

	// Check link density
	if metrics.LinkDensity > rules.MaxLinkDensity {
		metrics.FailureReasons = append(metrics.FailureReasons,
			"link density too high (potential spam)")
		passes = false
	}

	// Check ad/tracking scripts
	if metrics.AdScriptCount > rules.MaxAdScriptCount {
		metrics.FailureReasons = append(metrics.FailureReasons,
			"too many advertisement/tracking scripts")
		passes = false
	}

	// Check structured content
	if !metrics.HasParagraphs {
		metrics.FailureReasons = append(metrics.FailureReasons,
			"no paragraph structure found")
		passes = false
	}

	return passes
}

func isNumberOnly(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
