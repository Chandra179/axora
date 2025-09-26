package crawler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"axora/pkg/chunking"
	"axora/repository"

	"github.com/gocolly/colly/v2"
	"go.uber.org/zap"
)

type CrawlerConfig struct {
	MaxDepth        int
	RequestTimeout  time.Duration
	Parallelism     int
	IPRotationDelay time.Duration
	RequestDelay    time.Duration
	MaxRetries      int
	MaxURLVisits    int
	IPCheckServices []string
}

func DefaultConfig() *CrawlerConfig {
	return &CrawlerConfig{
		MaxDepth:        3,
		RequestTimeout:  10800 * time.Second,
		Parallelism:     10,
		IPRotationDelay: 40 * time.Second,
		RequestDelay:    5 * time.Second,
		MaxRetries:      3,
		MaxURLVisits:    1,
		IPCheckServices: []string{
			"https://httpbin.org/ip",
			"https://api.ipify.org?format=text",
			"https://icanhazip.com",
		},
	}
}

type ContextKey string

const (
	ContextIDKey ContextKey = "context_id"
	IPKey        ContextKey = "ip"
	LinkID       ContextKey = "link_id"
)

type Crawler struct {
	collector       *colly.Collector
	chunker         chunking.ChunkingClient
	crawlVectorRepo repository.CrawlVectorRepo
	extractor       *ContentExtractor
	config          *CrawlerConfig
	logger          *zap.Logger
	maxRetries      int
	torControlURL   string
	httpClient      http.Client
	downloadPath    string
	iPCheckServices []string
	torProxyUrl     string
	transport       *http.Transport
	delay           time.Duration
}

func NewCrawler(
	crawlVectorRepo repository.CrawlVectorRepo,
	extractor *ContentExtractor,
	chunker chunking.ChunkingClient,
	torProxyUrl string,
	torControlURL string,
	downloadPath string,
	logger *zap.Logger,
	config *CrawlerConfig,
) (*Crawler, error) {
	if config == nil {
		config = DefaultConfig()
	}
	proxyURL, _ := url.Parse(torProxyUrl)
	transport := &http.Transport{
		Proxy:             http.ProxyURL(proxyURL),
		DisableKeepAlives: true,
	}
	client := &http.Client{Transport: transport}

	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 "+
			"(KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.MaxDepth(config.MaxDepth),
		colly.Async(true),
		colly.TraceHTTP(),
		colly.ParseHTTPErrorResponse(),
		colly.URLFilters(
			regexp.MustCompile(`^https://libgen\.li(?:/(?:index\.php|edition\.php|ads\.php|get\.php))?(?:\?(?:.*(?:req|id|md5|downloadname|key|ext)=.*)?)?$`),
			regexp.MustCompile(`^https://[^.]+\.booksdl\.lc(?:/(?:index\.php|edition\.php|ads\.php|get\.php))?(?:\?(?:.*(?:id|md5)=.*)?)?$`),
		),
		// colly.Debugger(&debug.LogDebugger{}),
	)
	c.WithTransport(transport)
	c.SetClient(client)
	c.SetRequestTimeout(config.RequestTimeout)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: config.Parallelism,
		Delay:       config.RequestDelay,
	})

	worker := &Crawler{
		collector:       c,
		chunker:         chunker,
		crawlVectorRepo: crawlVectorRepo,
		extractor:       extractor,
		config:          config,
		logger:          logger,
		maxRetries:      config.MaxRetries,
		torControlURL:   torControlURL,
		httpClient:      *client,
		downloadPath:    downloadPath,
		iPCheckServices: config.IPCheckServices,
		torProxyUrl:     torProxyUrl,
		transport:       transport,
		delay:           config.IPRotationDelay,
	}

	return worker, nil
}

func (w *Crawler) Crawl(ctx context.Context, urls []string) error {
	contextId := GenerateContextID()
	ip := w.GetPublicIP(ctx)
	ctx = context.WithValue(ctx, ContextIDKey, contextId)
	ctx = context.WithValue(ctx, IPKey, ip)

	w.logger.With(
		zap.String(string(ContextIDKey), contextId),
		zap.String(string(IPKey), ip),
	)

	w.setupEventHandlers(ctx)
	for _, urlStr := range urls {
		if err := w.collector.Visit(urlStr); err != nil {
			w.logger.Error("Failed to visit URL",
				zap.String("url", urlStr),
				zap.Error(err))
		}
	}
	w.collector.Wait()
	w.logger.Info("Crawl session completed")

	return nil
}

func (w *Crawler) setupEventHandlers(ctx context.Context) {
	w.collector.OnHTML("a[href]", w.OnHTML(ctx))
	w.collector.OnError(w.OnError(ctx, w.collector))
	w.collector.OnResponse(w.OnResponse(ctx))
}

func GenerateContextID() string {
	randomBytes := make([]byte, 16)
	_, err := rand.Read(randomBytes)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(randomBytes)
}
