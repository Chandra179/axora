package crawler

import (
	"context"
	"net/http"
	"regexp"
	"time"

	"github.com/gocolly/colly/v2"
	"go.uber.org/zap"
)

type DownloadableURL struct {
	ID  string
	URL string
}

type CrawlDocClient interface {
	InsertOne(ctx context.Context, url string, isDownloadable bool, downloadStatus string) error
	UpdateDownloadStatus(ctx context.Context, id string, status string) error
	GetDownloadableUrls(ctx context.Context) ([]DownloadableURL, error)
}

type CrawlEvent interface {
	Publish(subject string, msg []byte) error
}

type ContextKey string

const (
	ContextIDKey ContextKey = "context_id"
	IPKey        ContextKey = "ip"
	LinkID       ContextKey = "link_id"
)

type Crawler struct {
	collector     *colly.Collector
	logger        *zap.Logger
	httpClient    http.Client
	proxyUrl      string
	keyword       string
	crawlDoc      CrawlDocClient
	crawlEvent    CrawlEvent
	hostBlacklist map[string]struct{}
	pathBlacklist []string
}

func NewCrawler(
	proxyUrl string,
	httpClient *http.Client,
	httpTransport *http.Transport,
	logger *zap.Logger,
	crawlDoc CrawlDocClient,
	crawlEvent CrawlEvent,
) (*Crawler, error) {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 "+
			"(KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		// colly.MaxDepth(1),
		colly.Async(true),
		colly.TraceHTTP(),
		colly.ParseHTTPErrorResponse(),
		colly.URLFilters(
			regexp.MustCompile(`^https://.*$`),
			regexp.MustCompile(`^https://libgen\.li/index\.php\?req=[^&]+$`),
			regexp.MustCompile(`^https://libgen\.li/edition\.php\?id=[^&]+$`),
			regexp.MustCompile(`^https://libgen\.li/ads\.php\?md5=[^&]+$`),
			regexp.MustCompile(`^https://libgen\.li/get\.php\?md5=[^&]+&key=[^&]+$`),
			regexp.MustCompile(`^https://[^.]+\.booksdl\.lc/get\.php\?md5=[^&]+(?:&key=[^&]+)?$`),
		),
		// colly.Debugger(&debug.LogDebugger{}),
	)

	c.WithTransport(httpTransport)
	c.SetClient(httpClient)
	c.SetRequestTimeout(30 * time.Second)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 30,
		Delay:       3 * time.Second,
	})
	c.IgnoreRobotsTxt = true

	worker := &Crawler{
		collector:  c,
		logger:     logger,
		httpClient: *httpClient,
		proxyUrl:   proxyUrl,
		crawlDoc:   crawlDoc,
		crawlEvent: crawlEvent,
		hostBlacklist: map[string]struct{}{
			"startpage": {},
			"brave":     {},
		},
		pathBlacklist: []string{"/about", "/help"},
	}

	return worker, nil
}

func (w *Crawler) Crawl(urls chan string, keyword string) error {
	w.collector.OnHTML("a[href]", w.OnHTML())
	// w.collector.OnHTML("body", w.OnHTMLDOMLog(ctx))
	w.collector.OnError(w.OnError(w.collector))
	w.collector.OnResponse(w.OnResponse())
	w.keyword = keyword

	for url := range urls {
		if err := w.collector.Visit(url); err != nil {
			w.logger.Error("Failed to visit URL",
				zap.String("url", url),
				zap.Error(err))
			continue
		}
	}
	w.collector.Wait()
	w.logger.Info("Crawl session completed")

	return nil
}
