package crawler

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"

	"axora/pkg/chunking"
	"axora/repository"

	"github.com/gocolly/colly/v2"
	"go.uber.org/zap"
	"golang.org/x/net/proxy"
)

type ContextKey string

const (
	ContextIDKey ContextKey = "context_id"
	IPKey        ContextKey = "ip"
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
	maxRetries      int
	torControlURL   string
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
		torControlURL:   torControlURL,
	}

	return worker, nil
}

// Crawl starts crawling the provided URLs
func (w *Worker) Crawl(ctx context.Context, urls []string) error {
	contextId := GenerateContextID()
	ip := w.ipChecker.GetPublicIP(ctx)
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

func (w *Worker) RotateIP() {
	conn, err := net.Dial("tcp", w.torControlURL)
	if err != nil {
		w.logger.Error("failed to connect to tor control port", zap.Error(err))
		return
	}
	defer conn.Close()

	// Authenticate with no password
	if _, err := fmt.Fprintf(conn, "AUTHENTICATE \"\"\r\n"); err != nil {
		w.logger.Error("failed to send authenticate command to tor", zap.Error(err))
		return
	}
	status, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		w.logger.Error("failed to read authentication status from tor", zap.Error(err))
		return
	}
	if status != "250 OK\r\n" {
		w.logger.Error("failed to authenticate with tor", zap.String("status", status))
		return
	}

	// Send NEWNYM signal
	if _, err := fmt.Fprintf(conn, "SIGNAL NEWNYM\r\n"); err != nil {
		w.logger.Error("failed to send NEWNYM signal to tor", zap.Error(err))
		return
	}
	status, err = bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		w.logger.Error("failed to read NEWNYM status from tor", zap.Error(err))
		return
	}
	if status != "250 OK\r\n" {
		w.logger.Error("failed to get new IP from tor", zap.String("status", status))
		return
	}

	w.logger.Info("successfully rotated IP address")
}

func GenerateContextID() string {
	randomBytes := make([]byte, 16)
	_, err := rand.Read(randomBytes)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(randomBytes)
}
