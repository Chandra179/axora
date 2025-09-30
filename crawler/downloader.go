package crawler

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type DownloadClient interface {
	DownloadFile(ctx context.Context, url, contentDisposition, expectedMD5 string) error
}

type DownloadMgr struct {
	maxFileNameLen int
	downloadPath   string
	logger         *zap.Logger
	httpClient     *http.Client
}

func NewDownloadMgr(logger *zap.Logger, downloadPath string, httpClient *http.Client) *DownloadMgr {
	return &DownloadMgr{
		logger:         logger,
		maxFileNameLen: 100,
		downloadPath:   downloadPath,
		httpClient:     httpClient,
	}
}

func (w *DownloadMgr) DownloadFile(ctx context.Context, url, contentDisposition, expectedMD5 string) error {
	fileName := w.extractFilenameFromHeader(contentDisposition)

	if fileName == "" {
		fileName = fmt.Sprintf("%d-%s", time.Now().UnixNano(), uuid.NewString())
	}

	fileName = w.truncateFilename(fileName)
	savePath := filepath.Join(w.downloadPath, fileName)
	currentIP, _ := GetPublicIP(ctx, w.httpClient)

	w.logger.Info("Starting file download",
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

	// Execute request
	resp, err := w.httpClient.Do(req)
	if err != nil {
		w.logger.Error("HTTP request failed", zap.Error(err))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	// Create the file
	out, err := os.Create(savePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", savePath, err)
	}
	defer out.Close()

	// Download the file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("copy error: %w", err)
	}

	w.logger.Info("File download completed", zap.String("save_path", savePath))

	// Validate MD5 only if expectedMD5 is provided
	if expectedMD5 != "" {
		if err := w.validateMD5(savePath, expectedMD5); err != nil {
			os.Remove(savePath)
			return err
		}
		w.logger.Info("MD5 verification successful", zap.String("md5", expectedMD5))
	} else {
		w.logger.Info("Skipping MD5 validation (no expected MD5 provided)")
	}

	return nil
}

// extractFilenameFromHeader extracts the filename from Content-Disposition header
// Returns empty string if not found or invalid
func (w *DownloadMgr) extractFilenameFromHeader(contentDisposition string) string {
	if !strings.Contains(contentDisposition, "filename=") {
		return ""
	}

	parts := strings.SplitN(contentDisposition, "filename=", 2)
	if len(parts) != 2 {
		return ""
	}

	filename := strings.Trim(parts[1], "\"")
	filename = strings.TrimSpace(filename)

	return filename
}

// truncateFilename truncates a filename if it exceeds maxFileNameLen
// Uses SHA1 hash to maintain uniqueness while shortening
func (w *DownloadMgr) truncateFilename(filename string) string {
	if len(filename) <= w.maxFileNameLen {
		return filename
	}

	hash := sha1.New()
	hash.Write([]byte(filename))
	hashStr := hex.EncodeToString(hash.Sum(nil))

	extension := filepath.Ext(filename)
	maxBaseLen := w.maxFileNameLen - len(extension) - 8 // 8 chars for hash

	if maxBaseLen < 0 {
		maxBaseLen = 0
	}

	base := filename[:maxBaseLen]
	return fmt.Sprintf("%s-%s%s", base, hashStr[:7], extension)
}

// validateMD5 verifies the MD5 hash of a downloaded file
func (w *DownloadMgr) validateMD5(filePath, expectedMD5 string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file for validation: %w", err)
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("failed to compute MD5: %w", err)
	}

	actualMD5 := fmt.Sprintf("%x", hash.Sum(nil))

	if !strings.EqualFold(actualMD5, expectedMD5) {
		return fmt.Errorf("MD5 verification failed: expected %s, got %s", expectedMD5, actualMD5)
	}

	return nil
}
