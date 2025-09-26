package crawler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"time"

	"axora/pkg/chunking"
	"axora/repository"

	"github.com/gocolly/colly/v2"
	"go.uber.org/zap"
)

type ContextKey string

const (
	ContextIDKey ContextKey = "context_id"
	IPKey        ContextKey = "ip"
	LinkID       ContextKey = "link_id"
)

type Worker struct {
	collector       *colly.Collector
	chunker         chunking.ChunkingClient
	crawlVectorRepo repository.CrawlVectorRepo
	extractor       *ContentExtractor
	validator       *URLValidator
	visitTracker    *VisitTracker
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

// NewWorker creates a new crawler worker with all dependencies
func NewWorker(
	crawlVectorRepo repository.CrawlVectorRepo,
	extractor *ContentExtractor,
	chunker chunking.ChunkingClient,
	torProxyUrl string,
	torControlURL string,
	downloadPath string,
	logger *zap.Logger,
	config *CrawlerConfig,
) (*Worker, error) {
	if config == nil {
		config = DefaultConfig()
	}
	proxyURL, _ := url.Parse(torProxyUrl)
	transport := &http.Transport{
		Proxy:             http.ProxyURL(proxyURL),
		DisableKeepAlives: true, // Force new connections
		MaxIdleConns:      0,    // Don't reuse connections
	}
	client := &http.Client{Transport: transport}

	validator := NewURLValidator(config)
	visitTracker := NewVisitTracker(config.MaxURLVisits)

	c := colly.NewCollector(
		colly.UserAgent(config.UserAgent),
		colly.MaxDepth(config.MaxDepth),
		colly.Async(true),
		colly.TraceHTTP(),
		colly.ParseHTTPErrorResponse(),
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

	worker := &Worker{
		collector:       c,
		chunker:         chunker,
		crawlVectorRepo: crawlVectorRepo,
		extractor:       extractor,
		validator:       validator,
		visitTracker:    visitTracker,
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

// Crawl starts crawling the provided URLs
func (w *Worker) Crawl(ctx context.Context, urls []string) error {
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
	w.logger.Info("Crawl session completed",
		zap.Int("total_visits", w.visitTracker.GetTotalVisits()),
		zap.Int("unique_urls", w.visitTracker.GetUniqueURLsCount()))

	return nil
}

// setupEventHandlers configures all colly event handlers
func (w *Worker) setupEventHandlers(ctx context.Context) {
	w.collector.OnRequest(w.OnRequest(ctx))
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
