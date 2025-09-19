package crawler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/textproto"
	"strings"
	"sync"
	"time"

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
	mu              sync.Mutex
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

	dialContext := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.Dial(network, addr)
	}
	transport := &http.Transport{DialContext: dialContext, DisableKeepAlives: false}
	client := &http.Client{Transport: transport}

	validator := NewURLValidator(config)
	visitTracker := NewVisitTracker(config.MaxURLVisits)

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
		validator:       validator,
		visitTracker:    visitTracker,
		config:          config,
		logger:          logger,
		maxRetries:      1,
		torControlURL:   torControlURL,
		httpClient:      *client,
		downloadPath:    downloadPath,
		iPCheckServices: config.IPCheckServices,
		torProxyUrl:     torProxyUrl,
		transport:       transport,
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

	fmt.Println("ip 1: ", ip)
	w.RotateIP()
	time.Sleep(60 * time.Second)
	fmt.Println("ip 2: ", w.GetPublicIP(ctx))
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

func (w *Worker) RotateIP() error {
	w.logger.Info("Attempting to rotate IP via Tor control")

	// Add timeout context for the entire operation
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Use DialContext with timeout
	d := net.Dialer{Timeout: 10 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", w.torControlURL)
	if err != nil {
		w.logger.Error("Failed to dial Tor control port", zap.Error(err))
		return fmt.Errorf("failed to dial Tor control port: %w", err)
	}
	defer func() {
		w.logger.Info("Closing Tor control connection")
		conn.Close()
	}()

	// Set connection deadline
	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		w.logger.Error("Failed to set connection deadline", zap.Error(err))
		return fmt.Errorf("failed to set connection deadline: %w", err)
	}

	w.logger.Info("Wrapping control connection in textproto")
	tp := textproto.NewConn(conn)
	defer func() {
		w.logger.Info("Closing textproto connection")
		tp.Close()
	}()

	w.logger.Info("Sending AUTHENTICATE command to Tor")
	if err := tp.PrintfLine(`AUTHENTICATE "%s"`, "noturpw321"); err != nil {
		w.logger.Error("Failed to send AUTHENTICATE command", zap.Error(err))
		return fmt.Errorf("failed to send AUTHENTICATE command: %w", err)
	}

	// Add small delay before reading response
	time.Sleep(100 * time.Millisecond)

	w.logger.Info("Reading AUTHENTICATE response")
	line, err := tp.ReadLine()
	if err != nil {
		w.logger.Error("Error reading AUTHENTICATE response", zap.Error(err))

		// Try to check if connection is still alive
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return fmt.Errorf("timeout reading AUTHENTICATE response: %w", err)
		}
		return fmt.Errorf("failed to read AUTHENTICATE response: %w", err)
	}

	w.logger.Info("AUTHENTICATE response received", zap.String("response", line))

	if !strings.HasPrefix(line, "250") {
		w.logger.Error("Authentication failed", zap.String("response", line))
		return fmt.Errorf("authentication failed with response: %s", line)
	}

	w.logger.Info("Authentication successful, sending SIGNAL NEWNYM")
	if err := tp.PrintfLine("SIGNAL NEWNYM"); err != nil {
		w.logger.Error("Failed to send SIGNAL NEWNYM", zap.Error(err))
		return fmt.Errorf("failed to send SIGNAL NEWNYM: %w", err)
	}

	// Add small delay before reading response
	time.Sleep(100 * time.Millisecond)

	w.logger.Info("Reading NEWNYM response")
	line, err = tp.ReadLine()
	if err != nil {
		w.logger.Error("Error reading NEWNYM response", zap.Error(err))
		return fmt.Errorf("failed to read NEWNYM response: %w", err)
	}

	w.logger.Info("NEWNYM response received", zap.String("response", line))

	if !strings.HasPrefix(line, "250") {
		w.logger.Error("NEWNYM command failed", zap.String("response", line))
		return fmt.Errorf("NEWNYM failed with response: %s", line)
	}

	// Wait a moment for Tor to establish new circuit
	w.logger.Info("Waiting for new Tor circuit to establish")
	time.Sleep(2 * time.Second)

	w.logger.Info("Creating new SOCKS5 dialer via", zap.String("proxy", w.torProxyUrl))
	dialer, err := proxy.SOCKS5("tcp", w.torProxyUrl, nil, proxy.Direct)
	if err != nil {
		w.logger.Error("Failed to create SOCKS5 dialer", zap.Error(err))
		return fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}

	w.logger.Info("Building new HTTP transport and client")
	dialContext := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.Dial(network, addr)
	}

	newTransport := &http.Transport{
		DialContext:           dialContext,
		DisableKeepAlives:     false,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	newClient := &http.Client{
		Transport: newTransport,
		Timeout:   30 * time.Second,
	}

	w.logger.Info("Swapping in new transport and client")
	w.mu.Lock()
	oldTransport := w.transport
	w.transport = newTransport
	w.httpClient = *newClient
	w.collector.SetClient(newClient)
	w.collector.WithTransport(newTransport)
	w.mu.Unlock()

	// Clean up old transport
	if oldTransport != nil {
		w.logger.Info("Closing old idle connections")
		oldTransport.CloseIdleConnections()
	}

	w.logger.Info("Successfully rotated IP via Tor")
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
