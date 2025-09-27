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
		isVisited, errV := w.collector.HasVisited(absoluteURL)
		if errV != nil {
			return
		}
		if isVisited {
			return
		}

		w.logger.Info("visiting: " + absoluteURL)
		e.Request.Visit(absoluteURL)
	}
}

// OnError handles error events with retry logic
func (w *Crawler) OnError(ctx context.Context, collector *colly.Collector) colly.ErrorCallback {
	return func(r *colly.Response, err error) {
		time.Sleep(w.config.IPRotationDelay)
		r.Request.Retry()
	}
}

// OnResponse handles successful responses and downloads
func (w *Crawler) OnResponse(ctx context.Context) colly.ResponseCallback {
	return func(r *colly.Response) {
		w.logger.Info("onrepsonse: " + r.Request.URL.String())
		contentType := r.Headers.Get("Content-Type")
		contentDisposition := r.Headers.Get("Content-Disposition")

		if !strings.Contains(strings.ToLower(contentDisposition), "attachment") &&
			contentType != "application/octet-stream" {
			return
		}

		go func(r *colly.Response) {
			filename := ExtractFilename(contentDisposition)
			u := r.Request.URL
			q := u.Query()
			md5hash := q.Get("md5")

			err := w.DownloadFile(ctx, u.String(), filename, md5hash)
			if err != nil {
				w.logger.Error("Download failed",
					zap.String("filename", filename),
					zap.Error(err))
			} else {
				w.logger.Info("Download completed successfully",
					zap.String("filename", filename),
					zap.String("md5", md5hash))
			}
		}(r)
	}
}
