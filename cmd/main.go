package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"axora/config"
	"axora/crawler"
	"axora/file"
	"axora/pkg/chunking"
	"axora/pkg/embedding"
	qdrantClient "axora/pkg/qdrantdb"

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
	logger, _ := zap.NewProduction()
	extractor := crawler.NewContentExtractor()
	mpnetbasev2 := embedding.NewMpnetBaseV2(cfg.AllMinilmL6V2URL)
	recurCharChunking := chunking.NewRecursiveCharacterChunking(mpnetbasev2)
	pdfPro := file.NewPDFExtractor(logger)
	epubPro := file.NewEpubExtractor(logger)
	fp := file.NewCore(pdfPro, epubPro, cfg.DownloadPath, logger)
	worker, err := crawler.NewCrawler(
		qdb,
		extractor,
		recurCharChunking,
		cfg.ProxyURL,
		cfg.TorControlURL,
		cfg.DownloadPath,
		logger,
		nil,
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
	http.Handle("/scrap", Crawl(worker, mpnetbasev2))
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(cfg.AppPort), nil))

}

func Crawl(worker *crawler.Crawler, embed embedding.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if strings.TrimSpace(query) == "" {
			http.Error(w, "missing q parameter", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Hour)
		defer cancel()

		worker.CollectUrls(ctx, query)
		// worker.Crawl(ctx, []string{"https://libgen.li/index.php?req=" + query + "%20ext:epub"})
		// worker.Crawl(ctx, []string{"https://libgen.li/ads.php?md5=893a98f863a22e2bca1e7db9a95a0089"})
		// worker.Crawl(ctx, []string{"https://libgen.li/index.php?req=" + query})
		// worker.Crawl(ctx, []string{"https://libgen.li/ads.php?md5=100e2484399564d365eb67b74077770d"})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Crawl started"))
	}
}
