package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
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

	h := func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if strings.TrimSpace(query) == "" {
			http.Error(w, "missing q parameter", http.StatusBadRequest)
			return
		}

		go func() {
			ctx1, cancel1 := context.WithTimeout(r.Context(), 3*time.Hour)
			defer cancel1()
			httpClient, httpTransport := NewHttpClient(cfg.ProxyURL)
			urls1, _ := browser.CollectUrls(ctx1, query+" filetype:epub")
			c1 := crawler.NewCrawler(
				cfg.ProxyURL,
				httpClient,
				httpTransport,
				logger,
				urls1,
				[]*regexp.Regexp{regexp.MustCompile(`^https://.*$`)},
				"browser",
				query)
			c1.Crawl(ctx1)
		}()

		go func() {
			ctx2, cancel2 := context.WithTimeout(r.Context(), 3*time.Hour)
			defer cancel2()
			httpClient2, httpTransport2 := NewHttpClient(cfg.ProxyURL)
			c2 := crawler.NewCrawler(
				cfg.ProxyURL,
				httpClient2,
				httpTransport2,
				logger,
				[]string{"https://libgen.li/index.php?req=" + query},
				[]*regexp.Regexp{regexp.MustCompile(`^https://libgen\.li/index\.php\?req=[^&]+$`),
					regexp.MustCompile(`^https://libgen\.li/edition\.php\?id=[^&]+$`),
					regexp.MustCompile(`^https://libgen\.li/ads\.php\?md5=[^&]+$`),
					regexp.MustCompile(`^https://libgen\.li/get\.php\?md5=[^&]+&key=[^&]+$`),
					regexp.MustCompile(`^https://[^.]+\.booksdl\.lc/get\.php\?md5=[^&]+&key=[^&]+$`)},
				"libgen",
				query)
			c2.Crawl(ctx2)
		}()

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
