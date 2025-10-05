package crawler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"
	"time"

	"github.com/gocolly/colly/v2"
	"go.uber.org/zap"
)

type ContextKey string

var BooksdlPattern = `^https://[^.]+\.booksdl\.lc/get\.php\?md5=[^&]+(?:&key=[^&]+)?$`

const (
	ContextIDKey ContextKey = "context_id"
	IPKey        ContextKey = "ip"
	LinkID       ContextKey = "link_id"
)

type Crawler struct {
	collector       *colly.Collector
	logger          *zap.Logger
	httpClient      http.Client
	proxyUrl        string
	IpRotationDelay time.Duration
	keyword         string
	crawlDoc        CrawlDocClient
}

func NewCrawler(
	proxyUrl string,
	httpClient *http.Client,
	httpTransport *http.Transport,
	logger *zap.Logger,
	crawlDoc CrawlDocClient,
) (*Crawler, error) {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 "+
			"(KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.MaxDepth(3),
		colly.Async(true),
		colly.TraceHTTP(),
		colly.ParseHTTPErrorResponse(),
		colly.URLFilters(
			regexp.MustCompile(`^https://.*$`),
			regexp.MustCompile(`^https://libgen\.li/index\.php\?req=[^&]+$`),
			regexp.MustCompile(`^https://libgen\.li/edition\.php\?id=[^&]+$`),
			regexp.MustCompile(`^https://libgen\.li/ads\.php\?md5=[^&]+$`),
			regexp.MustCompile(`^https://libgen\.li/get\.php\?md5=[^&]+&key=[^&]+$`),
			regexp.MustCompile(BooksdlPattern),
		),
		// colly.Debugger(&debug.LogDebugger{}),
	)

	c.WithTransport(httpTransport)
	c.SetClient(httpClient)
	c.SetRequestTimeout(180 * time.Minute)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 50,
		Delay:       3 * time.Second,
	})
	c.IgnoreRobotsTxt = true

	worker := &Crawler{
		collector:       c,
		logger:          logger,
		httpClient:      *httpClient,
		proxyUrl:        proxyUrl,
		IpRotationDelay: 40 * time.Second,
		crawlDoc:        crawlDoc,
	}

	return worker, nil
}

func (w *Crawler) Crawl(ctx context.Context, urls chan string, keyword string) error {
	w.logger.Info("start crawl")
	contextId := GenerateContextID()

	ip, err := GetPublicIP(ctx, &w.httpClient)
	if err != nil {
		return err
	}
	ctx = context.WithValue(ctx, ContextIDKey, contextId)
	ctx = context.WithValue(ctx, IPKey, ip)

	w.logger.With(
		zap.String(string(ContextIDKey), contextId),
		zap.String(string(IPKey), ip),
	)

	w.collector.OnHTML("a[href]", w.OnHTML(ctx))
	// w.collector.OnHTML("body", w.OnHTMLDOMLog(ctx))
	w.collector.OnError(w.OnError(ctx, w.collector))
	w.collector.OnResponse(w.OnResponse(ctx))
	w.keyword = keyword

	for url := range urls {
		if err := w.collector.Visit(url); err != nil {
			w.logger.Error("Failed to visit URL",
				zap.String("url", url),
				zap.Error(err))
			return err
		}
	}
	w.collector.Wait()
	w.logger.Info("Crawl session completed")

	return nil
}

func GenerateContextID() string {
	randomBytes := make([]byte, 16)
	_, err := rand.Read(randomBytes)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(randomBytes)
}
