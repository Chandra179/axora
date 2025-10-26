package main

import (
	"context"
	"log"
	"net/http"
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

func main() {
	// =========
	// Config
	// =========
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	domains := config.LoadDomains("/app/domains.yaml")

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
	chunkingClient, err := crawler.NewChunker(420, embeddingClient) // 512 token limit
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
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := crawlerInstance.Crawl(ch)
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
		q := r.URL.Query().Get("q")
		if strings.TrimSpace(q) == "" {
			http.Error(w, "missing q parameter", http.StatusBadRequest)
			return
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			err := crawlerInstance.Crawl(ch)
			if err != nil {
				logger.Info("err crawl: " + err.Error())
			}
		}()

		browser.CollectUrls(q, ch)
		if err != nil {
			logger.Info("error colect urls: " + err.Error())
		}

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
