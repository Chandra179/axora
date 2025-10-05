package crawler

import "context"

type DownloadableURL struct {
	ID  string
	URL string
}

type CrawlDocClient interface {
	InsertOne(ctx context.Context, url string, isDownloadable bool, downloadStatus string) error
	UpdateDownloadStatus(ctx context.Context, id string, status string) error
	GetDownloadableUrls(ctx context.Context) ([]DownloadableURL, error)
}
