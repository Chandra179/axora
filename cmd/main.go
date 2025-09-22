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
	pdfPro := file.NewPDFExtractor()
	epubPro := file.NewEPUBExtractor()
	fp := file.NewCore(pdfPro, epubPro, cfg.DownloadPath)
	worker, err := crawler.NewWorker(
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
	http.Handle("/scrap", Crawl(worker, mpnetbasev2))
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(cfg.AppPort), nil))

	fmt.Println("Running")
}

func Crawl(worker *crawler.Worker, embed embedding.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if strings.TrimSpace(query) == "" {
			http.Error(w, "missing q parameter", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Hour)
		defer cancel()

		worker.Crawl(ctx, []string{"https://libgen.li/index.php?req=" + query})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Crawl started"))
	}
}
