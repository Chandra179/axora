package crawler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/textproto"
	"os"
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
	w.logger.Info("Dialing Tor control port: " + w.torControlURL)
	conn, err := net.DialTimeout("tcp", w.torControlURL, 30*time.Second)
	if err != nil {
		w.logger.Error("Failed to dial Tor control port: " + err.Error())
		return err
	}
	defer func() {
		w.logger.Info("Closing Tor control connection")
		conn.Close()
	}()

	w.logger.Info("Wrapping control connection in textproto")
	tp := textproto.NewConn(conn)
	defer func() {
		w.logger.Info("Closing textproto connection")
		tp.Close()
	}()

	pass := os.Getenv("myStrongPassword")
	w.logger.Info("Sending AUTHENTICATE command to Tor")
	if err := tp.PrintfLine(`AUTHENTICATE "%s"`, pass); err != nil {
		w.logger.Error("Failed to send AUTHENTICATE: " + err.Error())
		return err
	}

	line, err := tp.ReadLine()
	if err != nil {
		w.logger.Error("Error reading AUTHENTICATE response: " + err.Error())
		return err
	}
	w.logger.Info("AUTHENTICATE response: " + line)
	if !strings.HasPrefix(line, "250") {
		return fmt.Errorf("auth failed: %s", line)
	}

	w.logger.Info("Sending SIGNAL NEWNYM to Tor")
	if err := tp.PrintfLine("SIGNAL NEWNYM"); err != nil {
		w.logger.Warn("Err creating SIGNAL NEWNYM: " + err.Error())
		return err
	}

	line, err = tp.ReadLine()
	if err != nil {
		w.logger.Error("Error reading NEWNYM response: " + err.Error())
		return err
	}
	w.logger.Info("NEWNYM response: " + line)
	if !strings.HasPrefix(line, "250") {
		return fmt.Errorf("NEWNYM failed: %s", line)
	}

	w.logger.Info("Creating SOCKS5 dialer via " + w.torProxyUrl)
	dialer, err := proxy.SOCKS5("tcp", w.torProxyUrl, nil, proxy.Direct)
	if err != nil {
		w.logger.Error("Failed to create SOCKS5 dialer: " + err.Error())
		return err
	}

	w.logger.Info("Building new http.Transport and http.Client")
	dialContext := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.Dial(network, addr)
	}

	newTransport := &http.Transport{
		DialContext:       dialContext,
		DisableKeepAlives: false, // keep-alives are okay if you close idle conns on swap
	}
	newClient := &http.Client{
		Transport: newTransport,
	}

	w.logger.Info("Swapping in new transport and client")
	w.mu.Lock()
	old := w.transport
	w.transport = newTransport
	w.collector.SetClient(newClient)
	w.collector.WithTransport(newTransport)
	w.mu.Unlock()

	if old != nil {
		w.logger.Info("Closing old idle connections")
		old.CloseIdleConnections()
	}

	w.logger.Info("SUCCESS: Rotated IP")
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
