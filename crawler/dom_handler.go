package crawler

import (
	"math"
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

		result, metrics, err := w.CleanHTML(r.Body)
		if err != nil {
			w.logger.Error("failed to clean HTML",
				zap.String("url", url),
				zap.Error(err))
			return
		}

		w.logger.Info("content_metrics",
			zap.String("url", url),
			zap.Int("word_count", metrics.WordCount),
			zap.Float64("text_html_ratio", math.Round(metrics.TextHTMLRatio*1000)/1000),
			zap.Int("sentence_count", metrics.SentenceCount),
			zap.Float64("avg_sentence_length", math.Round(metrics.AvgSentenceLength*100)/100),
			zap.Float64("vocab_richness", math.Round(metrics.VocabRichness*1000)/1000),
			zap.Float64("link_density", math.Round(metrics.LinkDensity*1000)/1000),
			zap.Int("ad_script_count", metrics.AdScriptCount),
			zap.Bool("has_paragraphs", metrics.HasParagraphs),
			zap.Bool("has_headings", metrics.HasHeadings),
			zap.Bool("passes_quality", metrics.PassesQualityCheck),
		)

		if !metrics.PassesQualityCheck {
			w.logger.Warn("content_quality_failed",
				zap.String("url", url),
				zap.Strings("reasons", metrics.FailureReasons))
			return
		}

		w.logger.Info("content_extracted",
			zap.String("url", url),
			zap.String("title", result.Metadata.Title),
			zap.String("author", result.Metadata.Author),
			zap.Time("date", result.Metadata.Date),
			zap.Int("content_length", len(result.ContentText)))
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
