package crawler

import (
	"net/url"
	"strings"

	"github.com/gocolly/colly/v2"
	"go.uber.org/zap"
)

// OnRequest handles request events
func (h *Worker) OnRequest(contextID, ip string) colly.RequestCallback {
	return func(r *colly.Request) {
		r.Ctx.Put("context_id", contextID)
		r.Ctx.Put("ip", ip)
		h.logger.With(
			zap.String("context_id", contextID),
			zap.String("ip", ip),
		)

		h.visitTracker.RecordVisit(r.URL.String())
	}
}

// OnHTML handles HTML parsing to find links
func (w *Worker) OnHTML() colly.HTMLCallback {
	return func(e *colly.HTMLElement) {
		link := e.Attr("href")
		absoluteURL := e.Request.AbsoluteURL(link)

		u, err := url.ParseRequestURI(absoluteURL)
		if err != nil {
			w.logger.Debug("Failed to parse URL",
				zap.String("url", absoluteURL),
				zap.Error(err))
			return
		}

		if !w.validator.IsValidDownloadURL(u) {
			return
		}

		if !w.visitTracker.ShouldVisit(absoluteURL) {
			return
		}

		e.Request.Visit(absoluteURL)
	}
}

// OnError handles error events with retry logic
func (w *Worker) OnError(collector *colly.Collector) colly.ErrorCallback {
	return func(r *colly.Response, err error) {
		var reqLogger *zap.Logger
		if r == nil {
			w.logger.Error("Request failed", zap.Error(err))
			return
		}

		reqLogger.Error("HTTP error",
			zap.String("url", r.Request.URL.String()),
			zap.Int("status_code", r.StatusCode),
			zap.Error(err))

		retryCount := r.Ctx.GetAny("retryCount")
		if retryCount == nil {
			r.Ctx.Put("retryCount", 1)
			retryCount = 1
		}
		rc := retryCount.(uint32)
		if rc < w.maxRetries {
			rc = rc + 1
			w.logger.Info("Retrying request",
				zap.String("url", r.Request.URL.String()),
				zap.Uint32("retry_attempt", rc))

			r.Ctx.Put("retryCount", rc)
			err := collector.Request("GET", r.Request.URL.String(), nil, r.Ctx, nil)
			if err != nil {
				w.logger.Error("Failed to resubmit request",
					zap.String("url", r.Request.URL.String()),
					zap.Error(err))
			}
		}
	}
}

// OnResponse handles successful responses and downloads
func (w *Worker) OnResponse() colly.ResponseCallback {
	return func(r *colly.Response) {
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

			err := w.downloader.DownloadFile(r.Ctx.Get("context_id"), u.String(), filename, md5hash)
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
