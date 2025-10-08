package crawler

import (
	"context"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"go.uber.org/zap"
)

func (w *Crawler) OnHTML() colly.HTMLCallback {
	return func(e *colly.HTMLElement) {
		// href := e.Attr("href")
		// absoluteURL := e.Request.AbsoluteURL(href)

		// e.Request.Visit(absoluteURL)
	}
}

func (w *Crawler) OnError(collector *colly.Collector) colly.ErrorCallback {
	return func(r *colly.Response, err error) {
		w.logger.Info("onerror: " + err.Error())
		time.Sleep(w.IpRotationDelay)
		r.Request.Retry()
	}
}

// Extract SLD (second-level domain) from host
func getSLD(host string) string {
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return host
	}
	return parts[len(parts)-2]
}

func (w *Crawler) OnResponse() colly.ResponseCallback {
	return func(r *colly.Response) {
		host := r.Request.URL.Host
		sld := getSLD(host)

		if _, blocked := w.hostBlacklist[sld]; blocked {
			return
		}

		path := r.Request.URL.Path
		for _, blocked := range w.pathBlacklist {
			if strings.Contains(path, blocked) {
				return
			}
		}

		url := r.Request.URL.String()
		if err := w.crawlEvent.Publish("web_crawl_tasks", []byte(url)); err != nil {
			w.logger.Error("Failed to publish URL",
				zap.String("url", url),
				zap.Error(err))
		}
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
