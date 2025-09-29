package crawler

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"errors"
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
	var fileName string
	if strings.Contains(contentDisposition, "filename=") {
		parts := strings.SplitN(contentDisposition, "filename=", 2)
		if len(parts) == 2 {
			name := strings.Trim(parts[1], "\"")
			if name != "" {
				fileName = fmt.Sprintf("%d-%s", time.Now().UnixNano(), uuid.NewString())
			}
		}
	}

	if len(fileName) > w.maxFileNameLen {
		hash := sha1.New()
		hash.Write([]byte(fileName))
		hashStr := hex.EncodeToString(hash.Sum(nil))
		extension := filepath.Ext(fileName)
		base := fileName[:w.maxFileNameLen-len(extension)-8]
		fileName = fmt.Sprintf("%s-%s%s", base, hashStr[:7], extension)
	}

	savePath := filepath.Join(w.downloadPath, fileName)
	if _, err := os.Stat(savePath); err == nil && expectedMD5 != "" {
		if err := w.ValidateDownload(savePath, expectedMD5); err == nil {
			w.logger.Info("File already exists and is valid, skipping download",
				zap.String("save_path", savePath))
			return nil
		}
	}

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

	resp, err := w.httpClient.Do(req)
	if err != nil {
		w.logger.Error("HTTP request failed", zap.Error(err))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	out, err := os.Create(savePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", savePath, err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("copy error: %w", err)
	}

	w.logger.Info("File download completed", zap.String("save_path", savePath))

	if err := w.ValidateDownload(savePath, expectedMD5); err != nil {
		os.Remove(savePath)
		return err
	}

	w.logger.Info("MD5 verification successful", zap.String("md5", expectedMD5))

	return nil
}

// ValidateDownload verifies the MD5 hash of a downloaded file
func (w *DownloadMgr) ValidateDownload(filePath, expectedMD5 string) error {
	if expectedMD5 == "" {
		return errors.New("md5 required: " + expectedMD5)
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
func (w *DownloadMgr) ExtractFilename(contentDisposition string) string {
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

	if len(filename) > w.maxFileNameLen {
		hash := sha1.New()
		hash.Write([]byte(filename))
		hashStr := hex.EncodeToString(hash.Sum(nil))
		extension := filepath.Ext(filename)
		base := filename[:w.maxFileNameLen-len(extension)-8]
		return fmt.Sprintf("%s-%s%s", base, hashStr[:7], extension)
	}
	return filename
}
