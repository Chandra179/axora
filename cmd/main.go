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

	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	logger, _ := zap.NewProduction()
	browser := crawler.NewBrowser(logger, cfg.ProxyURL)
	httpClient, httpTransport := NewHttpClient(cfg.ProxyURL)
	crawler, _ := crawler.NewCrawler(
		cfg.ProxyURL,
		httpClient,
		httpTransport,
		logger,
	)

	h := func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if strings.TrimSpace(q) == "" {
			http.Error(w, "missing q parameter", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Hour)
		defer cancel()

		libgenUrl := "https://libgen.li/index.php?req=" + q
		ch := make(chan string, 100)
		ch <- libgenUrl

		go func() {
			crawler.Crawl(ctx, ch, q)
		}()

		err := browser.CollectUrls(ctx, q+" filetype:epub", ch)
		if err != nil {
			fmt.Println("HAHAHA")
		}

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
