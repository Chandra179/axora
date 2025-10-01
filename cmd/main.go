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
	"axora/pkg/embedding"
	qdrantClient "axora/pkg/qdrantdb"
	textExtr "axora/text"

	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// ==========
	// DATABASE
	// ==========
	qdb, err := qdrantClient.NewClient(cfg.QdrantHost, cfg.QdrantPort)
	if err != nil {
		log.Fatalf("Failed to initialize Weaviate: %v", err)
	}
	err = qdb.CreateCrawlCollection(context.Background())
	if err != nil {
		log.Fatalf("err: %v", err)
	}

	// ==========
	// SERVICES
	// ==========
	httpClient, httpTransport := NewHttpClient(cfg.ProxyURL)
	logger, _ := zap.NewProduction()
	downloadMgr := crawler.NewDownloadMgr(logger, cfg.DownloadHost, cfg.ClamAvHost, httpClient)
	// extractor := crawler.NewContentExtractor()
	browser := crawler.NewBrowser(logger, cfg.ProxyURL)
	mpnetbasev2 := embedding.NewMpnetBaseV2(cfg.AllMinilmL6V2URL)
	// recurCharChunking := chunking.NewRecursiveCharacterChunking(mpnetbasev2)
	pdfPro := textExtr.NewPDFExtractor(logger)
	epubPro := textExtr.NewEpubExtractor(logger)
	fp := textExtr.NewCore(pdfPro, epubPro, cfg.DownloadHost, logger)
	worker, err := crawler.NewCrawler(
		cfg.ProxyURL,
		httpClient,
		httpTransport,
		logger,
		downloadMgr,
	)
	if err != nil {
		logger.Fatal("Failed to create worker", zap.Error(err))
	}

	// ==========
	// RUN
	// ==========
	go func() {
		fp.ProcessFiles()
	}()
	fmt.Println("Running")
	http.Handle("/scrap", Crawl(worker, *browser, mpnetbasev2))
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(cfg.AppPort), nil))

}

func Crawl(worker *crawler.Crawler, browser crawler.Browser, embed embedding.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if strings.TrimSpace(query) == "" {
			http.Error(w, "missing q parameter", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Hour)
		defer cancel()

		urls, _ := browser.CollectUrls(ctx, query+" filetype:epub")
		libgenUrl := "https://libgen.li/index.php?req=" + query
		urls = append(urls, libgenUrl)

		worker.Crawl(ctx, urls)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Crawl started"))
	}
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
