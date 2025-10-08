package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"axora/config"
	"axora/crawler"
	"axora/pkg/kafka"
	"axora/pkg/postgres"

	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	browser := crawler.NewBrowser(logger, cfg.ProxyURL)
	httpClient, httpTransport := NewHttpClient(cfg.ProxyURL)
	pg, err := postgres.NewClient(cfg.PostgresDBUrl)
	if err != nil {
		logger.Fatal("failed to create postgres client", zap.Error(err))
	}
	kafkaClient, err := kafka.NewClient(cfg.KafkaURL)
	if err != nil {
		logger.Fatal("failed to create nats client", zap.Error(err))
	}

	crawlerInstance, err := crawler.NewCrawler(
		cfg.ProxyURL,
		httpClient,
		httpTransport,
		logger,
		pg,
		kafkaClient,
	)
	if err != nil {
		logger.Fatal("failed to create crawler", zap.Error(err))
	}

	downloadManager, err := crawler.NewDownloadManager(cfg.DownloadPath, pg, logger, httpClient)
	if err != nil {
		logger.Fatal("failed to create download manager", zap.Error(err))
	}

	if err := downloadManager.Start(); err != nil {
		logger.Fatal("failed to start download manager", zap.Error(err))
	}

	h := func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if strings.TrimSpace(q) == "" {
			http.Error(w, "missing q parameter", http.StatusBadRequest)
			return
		}

		ch := make(chan string)

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()
			err := crawlerInstance.Crawl(ch, q)
			if err != nil {
				logger.Info("err crawl: " + err.Error())
			}
		}()

		// ch <- "https://libgen.vg/index.php?req=" + q

		wg.Add(1)
		go func() {
			defer wg.Done()
			browser.CollectUrls(q, ch)
			if err != nil {
				logger.Info("error colect urls: " + err.Error())
			}
		}()

		go func() {
			wg.Wait()
			close(ch)
		}()

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Crawl started"))
	}

	http.HandleFunc("/scrap", h)

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("starting server", zap.Int("port", cfg.AppPort))
		serverErrors <- http.ListenAndServe(":"+strconv.Itoa(cfg.AppPort), nil)
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		logger.Fatal("server error", zap.Error(err))

	case sig := <-shutdown:
		logger.Info("shutdown signal received", zap.String("signal", sig.String()))

		downloadManager.Stop()

		logger.Info("shutdown complete")
	}
}

func NewHttpClient(proxyUrl string) (*http.Client, *http.Transport) {
	proxyURL, _ := url.Parse(proxyUrl)
	transport := &http.Transport{
		Proxy:               http.ProxyURL(proxyURL),
		IdleConnTimeout:     30 * time.Second,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		// DisableKeepAlives: true,
	}
	client := &http.Client{Transport: transport}
	return client, transport
}
