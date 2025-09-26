package crawler

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"go.uber.org/zap"
)

func (w *Worker) OnHTML(ctx context.Context) colly.HTMLCallback {
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

		parsedURL, err := url.Parse(absoluteURL)
		if err != nil {
			w.logger.Warn("Failed to parse URL", zap.String("url", absoluteURL))
			return
		}
		if !w.validator.IsValidDownloadURL(parsedURL) {
			w.logger.Debug("URL validation failed", zap.String("url", absoluteURL))
			return
		}

		e.Request.Visit(absoluteURL)
	}
}

// OnError handles error events with retry logic
func (w *Worker) OnError(ctx context.Context, collector *colly.Collector) colly.ErrorCallback {
	return func(r *colly.Response, err error) {
		time.Sleep(w.config.IPRotationDelay)

		r.Request.Retry()
	}
}

// OnResponse handles successful responses and downloads
func (w *Worker) OnResponse(ctx context.Context) colly.ResponseCallback {
	return func(r *colly.Response) {
		w.logger.Info("onresponse: " + r.Request.URL.String())
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
