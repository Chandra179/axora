package crawler

import (
	"context"
	"net/http"

	"axora/pkg/chunking"
	"axora/repository"

	"github.com/gocolly/colly/v2"
	"go.uber.org/zap"
	"golang.org/x/net/proxy"
)

type Worker struct {
	collector       *colly.Collector
	chunker         chunking.ChunkingClient
	crawlVectorRepo repository.CrawlVectorRepo
	extractor       *ContentExtractor
	downloader      *FileDownloader
	validator       *URLValidator
	ipChecker       *IPChecker
	visitTracker    *VisitTracker
	config          *CrawlerConfig
	logger          *zap.Logger
	maxRetries      uint32
}

// NewWorker creates a new crawler worker with all dependencies
func NewWorker(
	crawlVectorRepo repository.CrawlVectorRepo,
	extractor *ContentExtractor,
	chunker chunking.ChunkingClient,
	torProxyUrl string,
	downloadPath string,
	logger *zap.Logger,
	config *CrawlerConfig,
) (*Worker, error) {
	if config == nil {
		config = DefaultConfig()
	}

	dialer, err := proxy.SOCKS5("tcp", torProxyUrl, nil, proxy.Direct)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{Dial: dialer.Dial}
	client := &http.Client{Transport: transport}

	ipChecker := NewIPChecker(*client, config.IPCheckServices, logger)
	validator := NewURLValidator(config)
	visitTracker := NewVisitTracker(config.MaxURLVisits)
	downloader := NewFileDownloader(*client, downloadPath, ipChecker, logger)

	c := colly.NewCollector(
		colly.UserAgent(config.UserAgent),
		colly.MaxDepth(config.MaxDepth),
		colly.Async(true),
	)
	c.WithTransport(transport)
	c.SetClient(client)
	c.SetRequestTimeout(config.RequestTimeout)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: config.Parallelism,
		Delay:       config.Delay,
	})

	worker := &Worker{
		collector:       c,
		chunker:         chunker,
		crawlVectorRepo: crawlVectorRepo,
		extractor:       extractor,
		downloader:      downloader,
		validator:       validator,
		ipChecker:       ipChecker,
		visitTracker:    visitTracker,
		config:          config,
		logger:          logger,
		maxRetries:      1,
	}

	return worker, nil
}

// Crawl starts crawling the provided URLs
func (w *Worker) Crawl(ctx context.Context, urls []string) error {
	contextID := GenerateContextID("crawl")
	currentIP := w.ipChecker.GetPublicIP(ctx)

	w.setupEventHandlers(contextID, currentIP)

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
func (w *Worker) setupEventHandlers(contextID, currentIP string) {
	w.collector.OnRequest(w.OnRequest(contextID, currentIP))
	w.collector.OnHTML("a[href]", w.OnHTML())
	w.collector.OnError(w.OnError(w.collector))
	w.collector.OnResponse(w.OnResponse())
}
