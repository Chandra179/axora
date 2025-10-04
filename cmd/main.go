package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"axora/config"
	"axora/crawler"
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

	// browser := crawler.NewBrowser(logger, cfg.ProxyURL)
	httpClient, httpTransport := NewHttpClient(cfg.ProxyURL)
	pg, err := postgres.NewClient(cfg.PostgresDBUrl)
	if err != nil {
		logger.Fatal("failed to create postgres client", zap.Error(err))
	}
	crawler, err := crawler.NewCrawler(
		cfg.ProxyURL,
		httpClient,
		httpTransport,
		logger,
		pg,
	)
	if err != nil {
		logger.Fatal("failed to create crawler", zap.Error(err))
	}

	h := func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if strings.TrimSpace(q) == "" {
			http.Error(w, "missing q parameter", http.StatusBadRequest)
			return
		}

		libgenUrl := "https://libgen.li/index.php?req=" + q
		ch := make(chan string, 500)
		ch <- libgenUrl

		go func() {
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Hour)
			defer cancel()
			crawler.Crawl(ctx, ch, q)
		}()

		// err := browser.CollectUrls(ctx, q+" filetype:epub", ch)
		// if err != nil {
		// 	fmt.Println("HAHAHA")
		// }

		close(ch)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Crawl started"))
	}

	fmt.Println("Running")
	http.HandleFunc("/scrap", h)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(cfg.AppPort), nil))

}

func NewHttpClient(proxyUrl string) (*http.Client, *http.Transport) {
	proxyURL, _ := url.Parse(proxyUrl)
	transport := &http.Transport{
		Proxy:             http.ProxyURL(proxyURL),
		DisableKeepAlives: true,
	}
	client := &http.Client{Transport: transport}
	return client, transport
}
