package crawler

import (
	"context"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"go.uber.org/zap"
)

// OnRequest handles request events
func (w *Worker) OnRequest(ctx context.Context) colly.RequestCallback {
	return func(r *colly.Request) {
		r.Ctx.Put(string(ContextIDKey), ctx.Value(ContextIDKey).(string))
		r.Ctx.Put(string(IPKey), ctx.Value(IPKey).(string))
		retryCount := r.Ctx.GetAny("retryCount")

		if retryCount == nil {
			r.Ctx.Put("retryCount", 0)
		}
		rc := r.Ctx.GetAny("retryCount").(int)
		if rc > w.maxRetries {
			r.Abort()
			return
		}

		if !w.validator.IsValidDownloadURL(r.URL) {
			r.Abort()
			return
		}

		if !w.visitTracker.ShouldVisit(r.URL.String()) {
			r.Abort()
			return
		}

		w.visitTracker.RecordVisit(r.URL.String())
	}
}

// OnHTML handles HTML parsing to find links
func (w *Worker) OnHTML(ctx context.Context) colly.HTMLCallback {
	return func(e *colly.HTMLElement) {
		link := e.Attr("href")
		absoluteURL := e.Request.AbsoluteURL(link)

		e.Request.Visit(absoluteURL)
	}
}

// OnError handles error events with retry logic
func (w *Worker) OnError(ctx context.Context, collector *colly.Collector) colly.ErrorCallback {
	return func(r *colly.Response, err error) {
		if r == nil {
			w.logger.Error("Request failed", zap.Error(err))
			return
		}
		retryCount := r.Ctx.GetAny("retryCount")
		if retryCount == nil {
			w.logger.Error("retry count should not be empty")
		}
		rc := r.Ctx.GetAny("retryCount").(int)

		rc = rc + 1
		r.Ctx.Put("retryCount", rc)

		w.logger.Info("Retrying request",
			zap.String("url", r.Request.URL.String()),
			zap.Int("retry_attempt", rc))

		time.Sleep(w.config.IPRotationDelay)
		err = collector.Request("GET", r.Request.URL.String(), nil, r.Ctx, nil)
		if err != nil {
			w.logger.Error("Failed to resubmit request",
				zap.String("url", r.Request.URL.String()),
				zap.Error(err))
		}
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
