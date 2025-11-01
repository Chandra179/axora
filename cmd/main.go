package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"strings"

	"net/url"
	"strconv"
	"time"

	"axora/config"
	"axora/crawler"
	"axora/pkg/embedding"
	qdrantClient "axora/pkg/qdrantdb"

	"go.uber.org/zap"
)

type SeedRequest struct {
	Topic          string `json:"topic"`
	ChunkingMethod string `json:"chunking_method"`
}

type BrowseRequest struct {
	Topic          string `json:"topic"`
	ChunkingMethod string `json:"chunking_method"`
}

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	// =========
	// Config
	// =========
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	domains := config.LoadDomains(cfg.DomainWhiteListPath)

	// =========
	// Logging
	// =========
	logger, errLog := zap.NewProduction()
	if errLog != nil {
		panic(errLog)
	}
	defer func() { _ = logger.Sync() }()

	// =========
	// Chromedp
	// =========
	browser := crawler.NewBrowser(logger, cfg.ProxyURL)

	// =========
	// HTTP
	// =========
	httpClient, httpTransport := NewHttpClient(cfg.ProxyURL)

	// =========
	// Qdrant vector
	// =========
	qdb, errQdrant := qdrantClient.NewClient(cfg.QdrantHost, cfg.QdrantPort)
	if errQdrant != nil {
		logger.Error("Failed to initialize qdrant", zap.Error(errQdrant))
	}
	err = qdb.CreateCrawlCollection(context.Background())
	if err != nil {
		logger.Error("Failed to initialize crawl collection", zap.Error(err))
	}

	// =========
	// Embedding Client
	// =========
	embeddingClient := embedding.NewMpnetBaseV2(cfg.MpnetBaseV2Url)

	// =========
	// Chunking Client
	// =========
	chunkingClient, errChunk := crawler.NewChunker(cfg.MaxEmbedModelTokenSize, embeddingClient,
		logger, cfg.TokenizerFilePath)
	if errChunk != nil {
		logger.Error("Failed to initialize chunk client", zap.Error(errChunk))
	}

	// =========
	// Crawler Service
	// =========
	crawlerInstance, errCrawl := crawler.NewCrawler(
		cfg.ProxyURL,
		httpClient,
		httpTransport,
		logger,
		qdb,
		chunkingClient,
		domains,
		cfg.BoltDBPath,
	)
	if errCrawl != nil {
		logger.Error("Failed to initialize crawl", zap.Error(errCrawl))
	}

	// =========
	// HTTP handler func
	// =========
	seedh := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req SeedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		if strings.TrimSpace(req.ChunkingMethod) == "" {
			http.Error(w, "missing chunking_method parameter", http.StatusBadRequest)
			return
		}

		ch := make(chan string)

		go func() {
			err := crawlerInstance.Crawl(ch, req.ChunkingMethod, req.Topic)
			if err != nil {
				logger.Error("crawl error", zap.Error(err))
			}
		}()

		ch <- "https://en.wikipedia.org/wiki/Economy"
		close(ch)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Crawl started"))
	}

	browseh := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req BrowseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		if strings.TrimSpace(req.Topic) == "" {
			http.Error(w, "missing topic parameter", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.ChunkingMethod) == "" {
			http.Error(w, "missing chunking_method parameter", http.StatusBadRequest)
			return
		}

		ch := make(chan string, 100)

		go func() {
			err := crawlerInstance.Crawl(ch, req.ChunkingMethod, req.Topic)
			if err != nil {
				logger.Error("crawl error", zap.Error(err))
			}
		}()

		browser.CollectUrls(req.Topic, ch)
		close(ch)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Crawl started"))
	}

	http.HandleFunc("/seed", seedh)
	http.HandleFunc("/browse", browseh)

	fmt.Println("serveeee")
	if err := http.ListenAndServe(":"+strconv.Itoa(cfg.AppPort), nil); err != nil {
		logger.Error("HTTP server failed", zap.Error(err))
	}
}

func NewHttpClient(proxyUrl string) (*http.Client, *http.Transport) {
	proxyURL, _ := url.Parse(proxyUrl)
	transport := &http.Transport{
		Proxy:                 http.ProxyURL(proxyURL),
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		ResponseHeaderTimeout: 120 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Minute,
	}

	return client, transport
}
