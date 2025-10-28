package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"strconv"
	"strings"
	"sync"
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
	// =========
	// Profiling
	// =========
	go func() {
		http.ListenAndServe(":6060", nil)
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
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync()

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
	qdb, err := qdrantClient.NewClient(cfg.QdrantHost, cfg.QdrantPort)
	if err != nil {
		log.Fatalf("Failed to initialize Weaviate: %v", err)
	}
	err = qdb.CreateCrawlCollection(context.Background())
	if err != nil {
		log.Fatalf("err: %v", err)
	}

	// =========
	// Embedding Client
	// =========
	embeddingClient := embedding.NewMpnetBaseV2(cfg.MpnetBaseV2Url)

	// =========
	// Chunking Client
	// =========
	chunkingClient, err := crawler.NewChunker(cfg.MaxEmbedModelTokenSize, embeddingClient,
		logger, cfg.TokenizerFilePath)
	if err != nil {
		log.Fatalf("Failed to initialize chunking client: %v", err)
	}

	// =========
	// Crawler Service
	// =========
	crawlerInstance, err := crawler.NewCrawler(
		cfg.ProxyURL,
		httpClient,
		httpTransport,
		logger,
		qdb,
		chunkingClient,
		domains,
	)
	if err != nil {
		logger.Fatal("failed to create crawler", zap.Error(err))
	}

	// =========
	// HTTP handler func
	// =========
	ch := make(chan string)
	var wg sync.WaitGroup
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

		wg.Add(1)
		go func() {
			defer wg.Done()
			err := crawlerInstance.Crawl(ch, req.ChunkingMethod, req.Topic)
			if err != nil {
				logger.Info("err crawl: " + err.Error())
			}
		}()

		ch <- "https://en.wikipedia.org/wiki/Economy"

		go func() {
			wg.Wait()
			close(ch)
		}()

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

		wg.Add(1)
		go func() {
			defer wg.Done()
			err := crawlerInstance.Crawl(ch, req.ChunkingMethod, req.Topic)
			if err != nil {
				logger.Info("err crawl: " + err.Error())
			}
		}()

		browser.CollectUrls(req.Topic, ch)

		go func() {
			wg.Wait()
			close(ch)
		}()

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Crawl started"))
	}

	http.HandleFunc("/seed", seedh)
	http.HandleFunc("/browse", browseh)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(cfg.AppPort), nil))
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
