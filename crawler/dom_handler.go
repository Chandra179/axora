package crawler

import (
	"strings"

	"github.com/gocolly/colly/v2"
	"go.uber.org/zap"
)

func (w *Crawler) OnHTML() colly.HTMLCallback {
	return func(e *colly.HTMLElement) {
		href := e.Attr("href")
		absoluteURL := e.Request.AbsoluteURL(href)
		e.Request.Visit(absoluteURL)
	}
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
