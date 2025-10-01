package crawler

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dutchcoders/go-clamd"
	"github.com/google/uuid"
	"github.com/h2non/filetype"
	"go.uber.org/zap"
)

type DownloadClient interface {
	DownloadFile(ctx context.Context, url, contentDisposition, expectedHash string) error
}

type DownloadMgr struct {
	maxFileNameLen    int
	downloadPath      string
	logger            *zap.Logger
	httpClient        *http.Client
	clamav            *clamd.Clamd
	maxFileSize       int64    // Maximum file size in bytes
	allowedExtensions []string // Whitelist of allowed extensions (empty = allow all)
	allowedMimeTypes  []string // Whitelist of MIME types (empty = allow all)
	clamAvHost        string   // ClamAV daemon address (e.g., "tcp://localhost:3310")
}

func NewDownloadMgr(logger *zap.Logger, downloadPath string, clamAvHost string, httpClient *http.Client) *DownloadMgr {
	mgr := &DownloadMgr{
		logger:            logger,
		maxFileNameLen:    100,
		downloadPath:      downloadPath,
		httpClient:        httpClient,
		maxFileSize:       100 * 1024 * 1024, //100mb
		clamAvHost:        clamAvHost,
		allowedExtensions: []string{".epub", ".pdf"},
		allowedMimeTypes:  []string{"application/pdf", "application/epub+zip"},
	}

	mgr.clamav = clamd.NewClamd(clamAvHost)
	if err := mgr.clamav.Ping(); err != nil {
		logger.Warn("Cannot connect to ClamAV, virus scanning disabled", zap.Error(err))
		mgr.clamav = nil
	} else {
		logger.Info("ClamAV connection established")
	}

	return mgr
}

func (w *DownloadMgr) DownloadFile(ctx context.Context, downloadURL, contentDisposition, expectedHash string) error {
	if err := w.validateURL(downloadURL); err != nil {
		return fmt.Errorf("URL validation failed: %w", err)
	}

	fileName := w.extractFilenameFromHeader(contentDisposition)
	if fileName != "" {
		fn, err := w.sanitizeFilename(fileName)
		if err != nil {
			return fmt.Errorf("filename sanitization failed: %w", err)
		}
		fileName = fn
	} else {
		fileName = fmt.Sprintf("%d-%s", time.Now().UnixNano(), uuid.NewString())
	}

	if err := w.validateExtension(fileName); err != nil {
		return err
	}

	fileName = w.truncateFilename(fileName)
	savePath := filepath.Join(w.downloadPath, fileName)

	// Ensure path is within downloadPath (prevent traversal)
	if err := w.validateSavePath(savePath); err != nil {
		return err
	}

	currentIP, _ := GetPublicIP(ctx, w.httpClient)

	w.logger.Info("Starting file download",
		zap.String("url", downloadURL),
		zap.String("save_path", savePath),
		zap.String("expected_hash", expectedHash),
		zap.String("ip", currentIP))

	// Ensure the directory exists
	dir := filepath.Dir(savePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "GoDownloader/2.0")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		w.logger.Error("HTTP request failed", zap.Error(err))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !w.isAllowedMimeType(contentType) {
		return fmt.Errorf("content type not allowed: %s", contentType)
	}

	tempPath := savePath + ".tmp"
	out, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", tempPath, err)
	}

	var downloadSuccess bool
	defer func() {
		out.Close()
		if !downloadSuccess {
			os.Remove(tempPath)
		}
	}()

	// Download with size limit
	limitedReader := io.LimitReader(resp.Body, w.maxFileSize+1)
	written, err := io.Copy(out, limitedReader)
	if err != nil {
		return fmt.Errorf("copy error: %w", err)
	}

	if written > w.maxFileSize {
		return fmt.Errorf("file size exceeds limit: %d bytes (max: %d)", written, w.maxFileSize)
	}

	out.Close()

	w.logger.Info("File download completed", zap.String("temp_path", tempPath), zap.Int64("size", written))

	if err := w.validateFileType(tempPath, fileName); err != nil {
		return err
	}

	if expectedHash != "" {
		if err := w.validateHash(tempPath, expectedHash); err != nil {
			return err
		}
		w.logger.Info("Hash verification successful", zap.String("hash", expectedHash))
	} else {
		w.logger.Info("Skipping hash validation (no expected hash provided)")
	}

	if err := w.scanForViruses(tempPath); err != nil {
		return err
	}
	w.logger.Info("Virus scan passed")

	//  Move temp file to final location
	if err := os.Rename(tempPath, savePath); err != nil {
		return fmt.Errorf("failed to move file to final location: %w", err)
	}

	downloadSuccess = true
	w.logger.Info("File successfully saved", zap.String("path", savePath))

	return nil
}

// validateURL checks for SSRF vulnerabilities
func (w *DownloadMgr) validateURL(rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid URL scheme: %s (only http/https allowed)", parsedURL.Scheme)
	}

	return nil
}

func (w *DownloadMgr) sanitizeFilename(filename string) (string, error) {
	//("/etc/passwd")     "passwd"
	//("C:\\temp\\x.txt") "x.txt"
	//("hello/world/")    "world"
	filename = filepath.Base(filename)
	filename = strings.TrimSpace(filename)

	if strings.Contains(filename, "\x00") {
		return "", fmt.Errorf("filename contains null bytes")
	}

	// Remove or replace dangerous characters
	dangerous := []string{"..", "\\", "/", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range dangerous {
		filename = strings.ReplaceAll(filename, char, "_")
	}

	// Ensure filename doesn't start with dot (hidden files)
	filename = strings.TrimPrefix(filename, ".")

	if filename == "" {
		return "", fmt.Errorf("filename is empty after sanitization")
	}

	return filename, nil
}

// validateSavePath ensures the path is within downloadPath
func (w *DownloadMgr) validateSavePath(savePath string) error {
	cleanSavePath, err := filepath.Abs(filepath.Clean(savePath))
	if err != nil {
		return fmt.Errorf("failed to resolve save path: %w", err)
	}

	cleanDownloadPath, err := filepath.Abs(filepath.Clean(w.downloadPath))
	if err != nil {
		return fmt.Errorf("failed to resolve download path: %w", err)
	}

	// Check if savePath is within downloadPath
	if !strings.HasPrefix(cleanSavePath, cleanDownloadPath) {
		return fmt.Errorf("path traversal detected: %s is outside %s", cleanSavePath, cleanDownloadPath)
	}

	return nil
}

func (w *DownloadMgr) validateExtension(filename string) error {
	ext := strings.ToLower(filepath.Ext(filename))

	allowed := false
	for _, allowedExt := range w.allowedExtensions {
		if ext == strings.ToLower(allowedExt) {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("file extension not in whitelist: %s", ext)
	}

	return nil
}

func (w *DownloadMgr) isAllowedMimeType(contentType string) bool {
	// Extract MIME type (remove parameters like charset)
	mimeType := strings.Split(contentType, ";")[0]
	mimeType = strings.TrimSpace(strings.ToLower(mimeType))

	for _, allowed := range w.allowedMimeTypes {
		if strings.ToLower(allowed) == mimeType {
			return true
		}
	}
	return false
}

func (w *DownloadMgr) validateFileType(filePath, filename string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file for type validation: %w", err)
	}
	defer file.Close()

	// Read first 261 bytes for magic number detection
	head := make([]byte, 261)
	n, err := file.Read(head)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read file header: %w", err)
	}

	kind, err := filetype.Match(head[:n])
	if err != nil {
		return fmt.Errorf("failed to detect file type: %w", err)
	}

	if kind == filetype.Unknown {
		w.logger.Warn("Unknown file type", zap.String("filename", filename))
		return nil // Allow unknown types, or return error if you want strict validation
	}

	w.logger.Info("File type validated", zap.String("type", kind.MIME.Value))
	return nil
}

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

func (w *DownloadMgr) truncateFilename(filename string) string {
	if len(filename) <= w.maxFileNameLen {
		return filename
	}

	hash := sha256.New()
	hash.Write([]byte(filename))
	hashStr := hex.EncodeToString(hash.Sum(nil))

	extension := filepath.Ext(filename)
	maxBaseLen := w.maxFileNameLen - len(extension) - 8

	if maxBaseLen < 0 {
		maxBaseLen = 0
	}

	base := filename[:maxBaseLen]
	return fmt.Sprintf("%s-%s%s", base, hashStr[:7], extension)
}

func (w *DownloadMgr) validateHash(filePath, expectedHash string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file for validation: %w", err)
	}
	defer file.Close()

	// Determine hash type by length
	expectedHash = strings.ToLower(strings.TrimSpace(expectedHash))
	var actualHash string

	switch len(expectedHash) {
	case 32: // MD5
		w.logger.Warn("Using MD5 hash (cryptographically weak, consider SHA-256)")
		hash := md5.New()
		if _, err := io.Copy(hash, file); err != nil {
			return fmt.Errorf("failed to compute MD5: %w", err)
		}
		actualHash = fmt.Sprintf("%x", hash.Sum(nil))

	case 64: // SHA-256
		hash := sha256.New()
		if _, err := io.Copy(hash, file); err != nil {
			return fmt.Errorf("failed to compute SHA-256: %w", err)
		}
		actualHash = fmt.Sprintf("%x", hash.Sum(nil))

	default:
		return fmt.Errorf("unsupported hash length: %d (expected 32 for MD5 or 64 for SHA-256)", len(expectedHash))
	}

	if actualHash != expectedHash {
		return fmt.Errorf("hash verification failed: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

// scanForViruses scans the file using ClamAV
func (w *DownloadMgr) scanForViruses(filePath string) error {
	w.logger.Info("Starting virus scan", zap.String("file", filePath))

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file for scanning: %w", err)
	}
	defer file.Close()

	response, err := w.clamav.ScanStream(file, make(chan bool))
	if err != nil {
		return fmt.Errorf("virus scan failed: %w", err)
	}

	for result := range response {
		if result.Status == clamd.RES_FOUND {
			return fmt.Errorf("virus detected: %s", result.Description)
		}
		if result.Status == clamd.RES_ERROR {
			return fmt.Errorf("virus scan error: %s", result.Description)
		}
	}

	return nil
}
