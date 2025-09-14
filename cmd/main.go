package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"axora/config"
	"axora/crawler"
	"axora/pkg/chunking"
	"axora/pkg/embedding"
	qdrantClient "axora/pkg/qdrantdb"
	"axora/pkg/tor"
	"axora/search"
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
	// TOR CLIENT
	// ==========
	torClient, err := tor.NewTorClient(cfg.TorProxyURL)
	if err != nil {
		log.Fatalf("Failed to initialize Tor client: %v", err)
	}

	if err := torClient.TestConnection(); err != nil {
		log.Printf("Warning: Tor connection test failed: %v", err)
	} else {
		log.Println("Tor connection established successfully")
	}

	// ==========
	// OTHER SERVICES
	// ==========
	extractor := crawler.NewContentExtractor()
	mpnetbasev2 := embedding.NewMpnetBaseV2(cfg.AllMinilmL6V2URL)
	search := search.NewSerpApiSearchEngine(cfg.SerpApiKey)
	recurCharChunking := chunking.NewRecursiveCharacterChunking(mpnetbasev2)

	// ==========
	// Crawler worker
	// ==========
	worker := crawler.NewWorker(qdb, extractor, recurCharChunking, torClient)

	// ==========
	// HTTP
	// ==========
	http.Handle("/search", Crawl(search, worker, mpnetbasev2))

	fmt.Println("Running")
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(cfg.AppPort), nil))
}

func Crawl(serp *search.SerpApiSearchEngine, worker *crawler.Worker, embed embedding.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type RequestBody struct {
			Query string `json:"query"`
		}

		var body RequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}

		if body.Query == "" {
			http.Error(w, "missing query field", http.StatusBadRequest)
			return
		}

		worker.Crawl(context.Background(), []string{"https://libgen.li/ads.php?md5=c86550a6a3ad8b49a33d09441fa995f6"})
	}
}
