package crawler

import (
	"context"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"go.uber.org/zap"
)

func (w *Crawler) OnHTML(ctx context.Context) colly.HTMLCallback {
	return func(e *colly.HTMLElement) {
		href := e.Attr("href")
		absoluteURL := e.Request.AbsoluteURL(href)

		e.Request.Visit(absoluteURL)
	}
}

func (w *Crawler) OnError(ctx context.Context, collector *colly.Collector) colly.ErrorCallback {
	return func(r *colly.Response, err error) {
		w.logger.Info("onerror: " + err.Error())
		time.Sleep(w.IpRotationDelay)
		r.Request.Retry()
	}
}

func (w *Crawler) OnResponse(ctx context.Context) colly.ResponseCallback {
	return func(r *colly.Response) {
		// url := r.Request.URL.String()

		// contentType := r.Headers.Get("Content-Type")
		// contentDisposition := r.Headers.Get("Content-Disposition")

		// isDownloadable :=
		// 	(strings.Contains(strings.ToLower(contentDisposition), "attachment") &&
		// 		contentType == "application/octet-stream") ||
		// 		contentType == "application/pdf"
	}
}

func (w *Crawler) OnHTMLDOMLog(ctx context.Context) colly.HTMLCallback {
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
			zap.Int("links_count", len(links)))
		// zap.Strings("links", links))
	}
}
