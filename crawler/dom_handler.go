package crawler

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/kljensen/snowball"
	"go.uber.org/zap"
)

func (w *Crawler) OnHTML(ctx context.Context) colly.HTMLCallback {
	return func(e *colly.HTMLElement) {
		href := e.Attr("href")
		absoluteURL := e.Request.AbsoluteURL(href)

		e.Request.Visit(absoluteURL)
	}
}

func (w *Crawler) OnHTMLDOMLog(ctx context.Context) colly.HTMLCallback {
	return func(e *colly.HTMLElement) {
		url := e.Request.URL.String()

		var links []string
		var bookLinks []string

		e.ForEach("a[href]", func(i int, link *colly.HTMLElement) {
			href := link.Attr("href")
			text := strings.TrimSpace(link.Text)
			if text == "" {
				text = "[no text]"
			}
			absoluteURL := e.Request.AbsoluteURL(href)
			links = append(links, absoluteURL+" ("+text+")")

			// Specifically look for book-related links
			if strings.Contains(href, "/file.php?id=") ||
				strings.Contains(href, "/ads.php?md5=") ||
				strings.Contains(href, "edition.php?id=") {
				bookLinks = append(bookLinks, absoluteURL+" ("+text+")")
			}
		})

		w.logger.Info("HTML DOM Structure",
			zap.String("url", url),
			zap.Int("links_count", len(links)),
			zap.Int("book_links_count", len(bookLinks)),
			zap.Strings("book_links", bookLinks))
	}
}

func (w *Crawler) OnError(ctx context.Context, collector *colly.Collector) colly.ErrorCallback {
	return func(r *colly.Response, err error) {
		time.Sleep(w.IpRotationDelay)
		r.Request.Retry()
	}
}

func (w *Crawler) OnResponse(ctx context.Context) colly.ResponseCallback {
	return func(r *colly.Response) {
		url := r.Request.URL.String()
		contentType := r.Headers.Get("Content-Type")
		contentDisposition := r.Headers.Get("Content-Disposition")

		isDownloadable := strings.Contains(strings.ToLower(contentDisposition), "attachment") &&
			contentType != "application/octet-stream"

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(r.Body)))
		if err != nil {
			w.logger.Error("failed parsing HTML: " + err.Error())
			return
		}

		title := doc.Find("title").Text()
		metaDesc, _ := doc.Find("meta[name=description]").Attr("content")
		searchable := strings.ToLower(title + " " + metaDesc)
		isCotainKeyword := containsStem(searchable, w.keyword)

		if isDownloadable {
			isMatch, err := regexp.MatchString(BooksdlPattern, url)
			if err != nil {
				w.logger.Info("error matching regex: " + err.Error())
			}
			if isMatch {
				w.crawlDoc.InsertOne(ctx, url, true, "pending")
				w.logger.Info("match1.0: " + r.Request.URL.String())
				return
			}
			if isCotainKeyword {
				w.crawlDoc.InsertOne(ctx, url, true, "pending")
				w.logger.Info("match1.1: " + r.Request.URL.String())
			}
			return
		}

		if isCotainKeyword {
			w.crawlDoc.InsertOne(ctx, url, false, "pending")
			w.logger.Info("match2: " + r.Request.URL.String())
		}
	}
}

func containsStem(text, keyword string) bool {
	words := strings.Fields(text)
	stemKeyword, _ := snowball.Stem(keyword, "english", true)
	for _, w := range words {
		stemWord, _ := snowball.Stem(w, "english", true)
		if stemWord == stemKeyword {
			return true
		}
	}
	return false
}
