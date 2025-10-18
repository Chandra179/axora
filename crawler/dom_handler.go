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

		isMetaRelevant := isMetaRelevant(doc, topic)
		title := doc.Find("title").Text()
		titleRelevant := isTopicRelevant(title, topic)

		if !isMetaRelevant || !titleRelevant {
			return
		}
		result, err := w.CleanHTML(r.Body, url)
		if err != nil {
			w.logger.Error("failed to clean HTML",
				zap.String("url", url),
				zap.Error(err))
			return
		}
		w.logger.Info("result",
			zap.String("url", url),
			zap.String("sitename", result.SiteName),
			zap.String("text", result.TextContent),
			zap.String("title", result.Title),
			zap.String("excerpt", result.Excerpt),
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

	// Calculate minimum prefix length for matching
	// Use at least 4 characters, or the full stem length if shorter
	minPrefixLen := 4
	if len(topicStem) < minPrefixLen {
		minPrefixLen = len(topicStem)
	}

	// early filter to avoid full tokenization if text clearly unrelated
	if len(topic) >= 3 && !strings.Contains(text, topic[:3]) {
		return false
	}

	words := strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == ',' || r == '.' || r == ';' || r == ':' || r == '!' || r == '?' || r == '\n'
	})

	for _, w := range words {
		if len(topic) >= 3 && !strings.Contains(w, topic[:3]) {
			continue
		}
		stem := stemWord(w)

		// Calculate the minimum length between the two stems
		compareLen := minPrefixLen
		if len(stem) < compareLen {
			compareLen = len(stem)
		}
		if len(topicStem) < compareLen {
			compareLen = len(topicStem)
		}

		// Match if stems share a common prefix
		if compareLen > 0 && compareLen >= minPrefixLen && stem[:compareLen] == topicStem[:compareLen] {
			return true
		}
	}
	return false
}

func isMetaRelevant(doc *goquery.Document, topic string) bool {
	var isRelevant bool

	metas := doc.Find("meta")
	for i := 0; i < metas.Length(); i++ {
		s := metas.Eq(i)
		name, _ := s.Attr("name")
		prop, _ := s.Attr("property")
		content, _ := s.Attr("content")

		if isTopicRelevant(name+prop+content, topic) {
			isRelevant = true
			break
		}
	}
	return isRelevant
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
