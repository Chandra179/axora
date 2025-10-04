package crawler

import "context"

type CrawlDocClient interface {
	InsertOne(ctx context.Context, url string, isDownloadable bool, downloadStatus string) error
}
