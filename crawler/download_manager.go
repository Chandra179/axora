package crawler

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	ChunkSize    = 1024 * 1024 // 1MB chunks
	MaxRetries   = 3
	RetryDelay   = time.Second * 2
	DownloadsDir = "./downloads"
)

type DownloadManager struct {
	client *http.Client
}

type DownloadProgress struct {
	Filename        string
	TotalBytes      int64
	DownloadedBytes int64
	LastOffset      int64
	MD5Expected     string
	URL             string
}

func NewDownloadManager() *DownloadManager {
	// Create downloads directory if it doesn't exist
	if err := os.MkdirAll(DownloadsDir, 0755); err != nil {
		fmt.Printf("Failed to create directory: %v", err)
	}

	return &DownloadManager{
		client: &http.Client{
			Timeout: time.Minute * 30,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if !strings.Contains(req.URL.Path, "get.php") {
					return nil
				}
				if !isValidRedirectURL(req.URL.String()) {
					return fmt.Errorf("invalid redirect URL: %s", req.URL.String())
				}

				// Check MD5 in redirect matches original
				if len(via) > 0 {
					originalMD5 := via[0].URL.Query().Get("md5")
					redirectMD5 := req.URL.Query().Get("md5")
					if originalMD5 != "" && redirectMD5 != "" && originalMD5 != redirectMD5 {
						return fmt.Errorf("MD5 mismatch in redirect: expected %s, got %s", originalMD5, redirectMD5)
					}
				}
				return nil
			},
		},
	}
}

// isValidRedirectURL validates redirect URLs according to specification
func isValidRedirectURL(redirectURL string) bool {
	u, err := url.Parse(redirectURL)
	if err != nil {
		return false
	}

	if u.Scheme != "https" {
		return false
	}

	// Must match pattern: https://cdn[1..n].booksdl.lc/get.php
	if !strings.HasSuffix(u.Host, ".booksdl.lc") {
		return false
	}

	if !strings.HasPrefix(u.Host, "cdn") {
		return false
	}

	if u.Path != "/get.php" {
		return false
	}

	return true
}

// validateDownloadResponse validates the response headers for download
func (dm *DownloadManager) validateDownloadResponse(resp *http.Response) error {
	contentDisposition := resp.Header.Get("Content-Disposition")
	if !strings.Contains(strings.ToLower(contentDisposition), "attachment") {
		return fmt.Errorf("Content-Disposition must contain attachment")
	}

	// Check Content-Range is not zero (for 206 partial responses)
	if resp.StatusCode == http.StatusPartialContent {
		contentRange := resp.Header.Get("Content-Range")
		if contentRange == "" {
			return fmt.Errorf("Content-Range missing for partial content")
		}
	}

	// Check Content-Type is supported
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/octet-stream" {
		log.Printf("[DOWNLOAD] Warning: Unexpected content type: %s", contentType)
	}

	return nil
}

// getFileSize gets the total file size from server
func (dm *DownloadManager) getFileSize(url string) (int64, error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := dm.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HEAD request failed with status: %d", resp.StatusCode)
	}

	contentLength := resp.Header.Get("Content-Length")
	if contentLength == "" {
		return 0, fmt.Errorf("Content-Length header missing")
	}

	size, err := strconv.ParseInt(contentLength, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid Content-Length: %s", contentLength)
	}

	return size, nil
}

// getLastOffset gets the last saved byte offset for resume capability
func (dm *DownloadManager) getLastOffset(filename string) int64 {
	filePath := filepath.Join(DownloadsDir, filename)
	info, err := os.Stat(filePath)
	if err != nil {
		return 0 // File doesn't exist, start from beginning
	}
	return info.Size()
}

// downloadChunk downloads a specific byte range
func (dm *DownloadManager) downloadChunk(url string, start, end int64, filename string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	// Set Range header for chunk download
	if end > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	} else {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", start))
	}

	resp, err := dm.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Validate response
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Validate download response headers
	if err := dm.validateDownloadResponse(resp); err != nil {
		return fmt.Errorf("response validation failed: %v", err)
	}

	// Open file for writing
	filePath := filepath.Join(DownloadsDir, filename)
	var file *os.File

	if start > 0 {
		// Resume mode - open for append
		file, err = os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	} else {
		// New download - create new file
		file, err = os.Create(filePath)
	}

	if err != nil {
		return err
	}
	defer file.Close()

	// Write chunks to file
	buffer := make([]byte, 32*1024) // 32KB buffer
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			_, writeErr := file.Write(buffer[:n])
			if writeErr != nil {
				return writeErr
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// verifyMD5 verifies the downloaded file's MD5 checksum
func (dm *DownloadManager) verifyMD5(filename, expectedMD5 string) error {
	if expectedMD5 == "" {
		log.Printf("[DOWNLOAD] No MD5 hash provided for verification of %s", filename)
		return nil
	}

	filePath := filepath.Join(DownloadsDir, filename)
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := md5.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return err
	}

	actualMD5 := fmt.Sprintf("%x", hash.Sum(nil))
	if actualMD5 != strings.ToLower(expectedMD5) {
		return fmt.Errorf("MD5 mismatch: expected %s, got %s", expectedMD5, actualMD5)
	}

	log.Printf("[DOWNLOAD] MD5 verification successful for %s", filename)
	return nil
}

// Download downloads a file with resume capability and MD5 verification
func (dm *DownloadManager) Download(url, filename, expectedMD5 string) error {
	log.Printf("[DOWNLOAD] Starting download: %s -> %s", url, filename)

	// Get file size
	totalSize, err := dm.getFileSize(url)
	if err != nil {
		return fmt.Errorf("failed to get file size: %v", err)
	}

	// Check if file already exists and get last offset
	lastOffset := dm.getLastOffset(filename)

	if lastOffset > 0 {
		log.Printf("[DOWNLOAD] Resuming download from byte %d", lastOffset)
		if lastOffset >= totalSize {
			log.Printf("[DOWNLOAD] File already complete, verifying MD5...")
			return dm.verifyMD5(filename, expectedMD5)
		}
	}

	// Download remaining bytes
	var lastErr error
	for retry := 0; retry < MaxRetries; retry++ {
		if retry > 0 {
			log.Printf("[DOWNLOAD] Retry %d/%d for %s", retry, MaxRetries, filename)
			time.Sleep(RetryDelay)
			// Recalculate last offset in case of partial download
			lastOffset = dm.getLastOffset(filename)
		}

		err = dm.downloadChunk(url, lastOffset, 0, filename)
		if err != nil {
			lastErr = err
			log.Printf("[DOWNLOAD] Download failed: %v", err)
			continue
		}

		// Download successful, verify MD5
		err = dm.verifyMD5(filename, expectedMD5)
		if err != nil {
			log.Printf("[DOWNLOAD] MD5 verification failed: %v", err)
			// Remove corrupted file
			os.Remove(filepath.Join(DownloadsDir, filename))
			lastErr = err
			continue
		}

		log.Printf("[DOWNLOAD] Download completed successfully: %s", filename)
		return nil
	}

	return fmt.Errorf("download failed after %d retries: %v", MaxRetries, lastErr)
}

// DownloadParallel downloads file using multiple parallel chunks (optional enhancement)
func (dm *DownloadManager) DownloadParallel(url, filename, expectedMD5 string, numChunks int) error {
	log.Printf("[DOWNLOAD] Starting parallel download with %d chunks: %s -> %s", numChunks, url, filename)

	totalSize, err := dm.getFileSize(url)
	if err != nil {
		return fmt.Errorf("failed to get file size: %v", err)
	}

	chunkSize := totalSize / int64(numChunks)
	if chunkSize < ChunkSize {
		// File too small for parallel download
		return dm.Download(url, filename, expectedMD5)
	}

	// Create temporary files for each chunk
	var wg sync.WaitGroup
	errors := make(chan error, numChunks)

	for i := 0; i < numChunks; i++ {
		wg.Add(1)
		go func(chunkIndex int) {
			defer wg.Done()

			start := int64(chunkIndex) * chunkSize
			var end int64
			if chunkIndex == numChunks-1 {
				end = totalSize - 1
			} else {
				end = start + chunkSize - 1
			}

			chunkFilename := fmt.Sprintf("%s.part%d", filename, chunkIndex)
			err := dm.downloadChunk(url, start, end, chunkFilename)
			if err != nil {
				errors <- fmt.Errorf("chunk %d failed: %v", chunkIndex, err)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		if err != nil {
			return err
		}
	}

	// Merge chunks
	finalPath := filepath.Join(DownloadsDir, filename)
	finalFile, err := os.Create(finalPath)
	if err != nil {
		return err
	}
	defer finalFile.Close()

	for i := 0; i < numChunks; i++ {
		chunkFilename := fmt.Sprintf("%s.part%d", filename, i)
		chunkPath := filepath.Join(DownloadsDir, chunkFilename)

		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			return err
		}

		_, err = io.Copy(finalFile, chunkFile)
		chunkFile.Close()
		os.Remove(chunkPath) // Clean up chunk file

		if err != nil {
			return err
		}
	}

	// Verify MD5
	return dm.verifyMD5(filename, expectedMD5)
}
