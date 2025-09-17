package crawler

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

type FileDownloader struct {
	httpClient   http.Client
	downloadPath string
	logger       *zap.Logger
	ipChecker    *IPChecker
}

// NewFileDownloader creates a new file downloader
func NewFileDownloader(client http.Client, downloadPath string, ipChecker *IPChecker, logger *zap.Logger) *FileDownloader {
	return &FileDownloader{
		httpClient:   client,
		downloadPath: downloadPath,
		ipChecker:    ipChecker,
		logger:       logger,
	}
}

// DownloadFile downloads a file from the given URL to the specified path
func (d *FileDownloader) DownloadFile(contextId, url, filename, expectedMD5 string) error {
	if expectedMD5 == "" {
		return nil
	}
	var currentIP string
	ctx := context.Background()
	ctx = context.WithValue(ctx, "context_id", contextId)

	if d.ipChecker != nil {
		currentIP = d.ipChecker.GetPublicIP(ctx)
	}

	savePath := filepath.Join(d.downloadPath, filename)

	d.logger.Info("Starting file download",
		zap.String("url", url),
		zap.String("save_path", savePath),
		zap.String("expected_md5", expectedMD5),
		zap.String("ip", currentIP))

	// Ensure the directory exists
	dir := filepath.Dir(savePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "GoDownloader/1.0")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		d.logger.Error("HTTP request failed", zap.Error(err))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var out *os.File

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("copy error: %w", err)
	}

	d.logger.Info("File download completed",
		zap.String("save_path", savePath))

	if err := d.ValidateDownload(savePath, expectedMD5); err != nil {
		os.Remove(savePath)
		return err
	}

	d.logger.Info("MD5 verification successful",
		zap.String("md5", expectedMD5))

	return nil
}

// ValidateDownload verifies the MD5 hash of a downloaded file
func (d *FileDownloader) ValidateDownload(filePath, expectedMD5 string) error {
	if expectedMD5 == "" {
		return nil // No validation needed
	}

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	actualMD5 := fmt.Sprintf("%x", hash.Sum(nil))

	if !strings.EqualFold(actualMD5, expectedMD5) {
		return fmt.Errorf("MD5 verification failed: expected %s, got %s", expectedMD5, actualMD5)
	}

	return nil
}

// ExtractFilename extracts filename from Content-Disposition header
func ExtractFilename(contentDisposition string) string {
	defaultName := "download.bin"

	if !strings.Contains(contentDisposition, "filename=") {
		return defaultName
	}

	parts := strings.Split(contentDisposition, "filename=")
	if len(parts) < 2 {
		return defaultName
	}

	filename := strings.Trim(parts[1], "\"")
	if filename == "" {
		return defaultName
	}

	return filename
}
