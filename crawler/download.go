package crawler

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/cavaliergopher/grab/v3"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type DownloadManager struct {
	crawlDoc     CrawlDocClient
	logger       *zap.Logger
	cron         *cron.Cron
	grabClient   *grab.Client
	downloadPath string
}

func NewDownloadManager(downloadPath string, crawlDoc CrawlDocClient,
	logger *zap.Logger, httpClient *http.Client) (*DownloadManager, error) {
	if err := os.MkdirAll(downloadPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create downloads directory: %w", err)
	}

	grabClient := grab.NewClient()
	grabClient.HTTPClient = httpClient
	grabClient.UserAgent = "CrawlDoc-Downloader/1.0"

	dm := &DownloadManager{
		crawlDoc:     crawlDoc,
		logger:       logger,
		cron:         cron.New(),
		grabClient:   grabClient,
		downloadPath: downloadPath,
	}

	return dm, nil
}

func (dm *DownloadManager) Start() error {
	_, err := dm.cron.AddFunc("*/5 * * * *", func() {
		ctx := context.Background()
		if err := dm.processDownloads(ctx); err != nil {
			dm.logger.Error("failed to process downloads", zap.Error(err))
		}
	})
	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	dm.cron.Start()
	dm.logger.Info("download manager cron job started")
	return nil
}

func (dm *DownloadManager) Stop() {
	if dm.cron != nil {
		dm.cron.Stop()
		dm.logger.Info("download manager cron job stopped")
	}
}

func (dm *DownloadManager) processDownloads(ctx context.Context) error {
	urls, err := dm.crawlDoc.GetDownloadableUrls(ctx)
	if err != nil {
		return fmt.Errorf("failed to get downloadable URLs: %w", err)
	}

	if len(urls) == 0 {
		dm.logger.Info("no downloadable URLs found")
		return nil
	}

	dm.logger.Info("found downloadable URLs", zap.Int("count", len(urls)))

	// TODO: could be using goroutine with limit
	for _, urlData := range urls {
		if err := dm.download(ctx, urlData.URL, urlData.ID); err != nil {
			dm.logger.Error("failed to download",
				zap.String("url", urlData.URL),
				zap.String("id", urlData.ID),
				zap.Error(err))

			if err := dm.crawlDoc.UpdateDownloadStatus(ctx, urlData.ID, "failed"); err != nil {
				dm.logger.Error("failed to update status to failed", zap.Error(err))
			}
			continue
		}

		dm.logger.Info("download completed successfully",
			zap.String("url", urlData.URL),
			zap.String("id", urlData.ID))
	}

	return nil
}

func (dm *DownloadManager) download(ctx context.Context, url, id string) error {
	dm.logger.Info("starting download", zap.String("url", url), zap.String("id", id))

	req, err := grab.NewRequest(dm.downloadPath, url)
	if err != nil {
		return fmt.Errorf("failed to create grab request: %w", err)
	}

	// Set filename if we want to customize it
	// The filename will be determined from Content-Disposition or URL
	req.NoCreateDirectories = true

	resp := dm.grabClient.Do(req)

	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			dm.logger.Debug("download progress",
				zap.String("id", id),
				zap.Float64("progress", resp.Progress()*100),
				zap.Int64("bytes_complete", resp.BytesComplete()),
				zap.Int64("bytes_total", resp.Size()),
				zap.Float64("speed_bps", resp.BytesPerSecond()))

		case <-resp.Done:
			if err := resp.Err(); err != nil {
				return fmt.Errorf("download failed: %w", err)
			}

			dm.logger.Info("file downloaded",
				zap.String("path", resp.Filename),
				zap.Int64("size", resp.Size()),
				zap.Duration("duration", resp.Duration()))

			if err := dm.crawlDoc.UpdateDownloadStatus(ctx, id, "completed"); err != nil {
				return fmt.Errorf("failed to update status to completed: %w", err)
			}

			return nil
		}
	}
}
