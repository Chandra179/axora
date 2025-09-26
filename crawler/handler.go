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
		e.ForEachWithBreak("a[href]", func(i int, link *colly.HTMLElement) bool {
			href := link.Attr("href")
			if href == "" {
				return true
			}
			absoluteURL := e.Request.AbsoluteURL(href)
			parsedURL, err := url.Parse(absoluteURL)
			isVisited, errV := w.collector.HasVisited(absoluteURL)
			if errV != nil {
				return true
			}
			if isVisited {
				return true
			}

			if err != nil {
				w.logger.Warn("Failed to parse URL", zap.String("url", absoluteURL))
				return true
			}

			if !w.validator.IsValidDownloadURL(parsedURL) {
				w.logger.Debug("URL validation failed", zap.String("url", absoluteURL))
				return true
			}

			err = e.Request.Visit(absoluteURL)
			if err != nil {
				w.logger.Error("Failed to visit URL",
					zap.String("url", absoluteURL),
					zap.Error(err))
			}

			return true
		})
	}
}

// OnError handles error events with retry logic
func (w *Worker) OnError(ctx context.Context, collector *colly.Collector) colly.ErrorCallback {
	return func(r *colly.Response, err error) {
		time.Sleep(w.config.IPRotationDelay)
		retryCount := r.Ctx.GetAny("retryCount")
		if retryCount == nil {
			w.logger.Error("retry count should not be empty")
			return
		}
		rc := retryCount.(int)

		if rc >= w.maxRetries {
			w.logger.Error("Max retries exceeded",
				zap.String("url", r.Request.URL.String()),
				zap.Int("retries", rc),
				zap.Error(err))
			return
		}

		r.Ctx.Put("retryCount", rc+1)

		w.logger.Info("Retrying request",
			zap.String("url", r.Request.URL.String()),
			zap.Int("retry_attempt", rc+1),
			zap.Int("max_retries", w.maxRetries),
			zap.Error(err))

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
