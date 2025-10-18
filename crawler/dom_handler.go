package crawler

import (
	"bytes"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/kljensen/snowball"
	"go.uber.org/zap"
)

func (w *Crawler) OnHTML() colly.HTMLCallback {
	return func(e *colly.HTMLElement) {
		href := e.Attr("href")
		absoluteURL := e.Request.AbsoluteURL(href)
		if shouldSkipURL(absoluteURL) {
			w.logger.Debug("skipping low-value URL", zap.String("url", absoluteURL))
			return
		}
		e.Request.Visit(absoluteURL)
	}
}

var skipPattern = regexp.MustCompile(`(?i)(contact|privacy|terms|faq|tag|archive|about|signin|login|register|
subscribe|feedback|cookies|sitemap|help|introduction|portal|events|community|search|changes|contribution)`)

func shouldSkipURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	path := strings.ToLower(parsed.Path)
	path = strings.ReplaceAll(path, "_", "-")
	path = strings.ReplaceAll(path, ".", "-")

	return skipPattern.MatchString(path)
}

func (w *Crawler) OnError(collector *colly.Collector) colly.ErrorCallback {
	return func(r *colly.Response, err error) {
		w.logger.Info("onerror: " + err.Error())
	}
}

func (w *Crawler) OnResponse() colly.ResponseCallback {
	return func(r *colly.Response) {
		url := r.Request.URL.String()
		w.logger.Info("url_log", zap.String("url", url))

		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(r.Body))
		if err != nil {
			w.logger.Error("failed to parse document", zap.String("url", url), zap.Error(err))
			return
		}

		topic := "economy"

		metaText := extractMetaText(doc, topic)
		title := strings.ToLower(doc.Find("title").Text())

		metaRelevant := isTopicRelevant(metaText, topic)
		titleRelevant := isTopicRelevant(title, topic)

		if !metaRelevant && !titleRelevant {
			w.logger.Info("skipped_non_topic_page",
				zap.String("url", url),
				zap.String("reason", "no relevant keyword in title/meta"))
			return
		}
		result, err := w.CleanHTML(r.Body, url)
		if err != nil {
			w.logger.Error("failed to clean HTML",
				zap.String("url", url),
				zap.Error(err))
			return
		}
		w.logger.Info("clean_result",
			zap.String("sitename", result.SiteName),
			zap.String("text", result.TextContent),
			zap.String("title", result.Title),
			zap.String("excerpt", result.Excerpt),
			zap.Int("length", result.Length),
		)
	}
}

func stemWord(word string) string {
	stem, err := snowball.Stem(word, "english", true)
	if err != nil {
		return word
	}
	return stem
}

func isTopicRelevant(text, topic string) bool {
	text = strings.ToLower(text)
	topicStem := stemWord(topic)

	// early filter to avoid full tokenization if text clearly unrelated
	if !strings.Contains(text, topic[:3]) {
		return false
	}

	words := strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == ',' || r == '.' || r == ';' || r == ':' || r == '!' || r == '?' || r == '\n'
	})

	for _, w := range words {
		if !strings.Contains(w, topic[:3]) {
			continue
		}
		stem := stemWord(w)
		if stem == topicStem {
			return true
		}
	}
	return false
}

func extractMetaText(doc *goquery.Document, topic string) string {
	var builder strings.Builder

	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		name, _ := s.Attr("name")
		prop, _ := s.Attr("property")
		content, _ := s.Attr("content")

		if content == "" {
			return
		}

		name = strings.ToLower(name)
		prop = strings.ToLower(prop)
		content = strings.ToLower(content)

		if isTopicRelevant(name, topic) ||
			isTopicRelevant(prop, topic) ||
			isTopicRelevant(content, topic) {
			builder.WriteString(" ")
			builder.WriteString(content)
		}
	})

	return builder.String()
}

func (w *Crawler) OnHTMLDOMLog() colly.HTMLCallback {
	return func(e *colly.HTMLElement) {
		url := e.Request.URL.String()

		var links []string

		e.ForEach("a[href]", func(i int, link *colly.HTMLElement) {
			href := link.Attr("href")
			text := strings.TrimSpace(link.Text)
			if text == "" {
				text = "[no text]"
			}
			absoluteURL := e.Request.AbsoluteURL(href)
			links = append(links, absoluteURL+" ("+text+")")
		})

		w.logger.Info("HTML DOM Structure",
			zap.String("url", url),
			zap.Strings("links", links),
			zap.Int("links_count", len(links)))
		// zap.Strings("links", links))
	}
}
