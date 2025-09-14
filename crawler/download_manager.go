package crawler

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"axora/pkg/tor"
)

const (
	ChunkSize    = 1024 * 1024 // 1MB chunks
	MaxRetries   = 3
	RetryDelay   = time.Second * 2
	DownloadsDir = "./downloads"
	tmpSuffix    = ".download.tmp"
)

type DownloadManager struct {
	tor *tor.TorClient
	hc  http.Client
	sem chan struct{}
}

func NewDownloadManager(tc *tor.TorClient) *DownloadManager {
	if err := os.MkdirAll(DownloadsDir, 0755); err != nil {
		fmt.Printf("Failed to create directory: %v\n", err)
	}
	httpClient := http.Client{
		Transport: &http.Transport{
			DialContext: tc.GetDialContext(),
		},
		Timeout:       time.Minute * 30,
		CheckRedirect: safeRedirectChecker([]string{".booksdl.lc"}),
	}
	return &DownloadManager{
		tor: tc,
		sem: make(chan struct{}, 3),
		hc:  httpClient,
	}
}

func (dm *DownloadManager) Download(rawurl, filename, expectedMD5 string) error {
	filename = sanitizeFilename(filename)

	dm.sem <- struct{}{}
	defer func() { <-dm.sem }()

	// Get total size via HEAD; if HEAD fails, attempt chunking until server closes.
	totalSize, headErr := dm.getFileSize(rawurl)
	if headErr != nil {
		log.Printf("[DOWNLOAD] HEAD failed for %s: %v (will attempt streaming)", rawurl, headErr)
		totalSize = -1
	}

	// Resume from tmp file offset
	offset := dm.getLastOffset(filename)

	// If offset == totalSize, already complete (perform MD5 verify)
	if totalSize > 0 && offset >= totalSize {
		tmpPath := filepath.Join(DownloadsDir, filename+tmpSuffix)
		if err := dm.verifyMD5(tmpPath, expectedMD5); err != nil {
			// remove and restart
			os.Remove(tmpPath)
			offset = 0
		} else {
			// promote to final
			finalPath := filepath.Join(DownloadsDir, filename)
			if err := os.Rename(tmpPath, finalPath); err != nil {
				return fmt.Errorf("rename failed: %w", err)
			}
			log.Printf("[DOWNLOAD] already complete: %s", finalPath)
			return nil
		}
	}

	// Loop over chunks until done
	for {
		// Determine chunk range
		start := offset
		var end int64 = 0
		if totalSize > 0 {
			if start >= totalSize {
				break
			}
			chunkEnd := start + ChunkSize - 1
			if chunkEnd >= totalSize {
				chunkEnd = totalSize - 1
			}
			end = chunkEnd
		} else {
			// unknown total size, request ChunkSize chunk (server may ignore Range)
			end = start + ChunkSize - 1
		}

		// Attempt chunk download with retries
		var lastErr error
		success := false
		for attempt := 0; attempt < MaxRetries; attempt++ {
			if attempt > 0 {
				time.Sleep(RetryDelay)
			}
			if err := dm.downloadChunk(rawurl, start, end, filename); err != nil {
				lastErr = err
				log.Printf("[DOWNLOAD] chunk attempt %d failed for %s (%d-%d): %v", attempt+1, rawurl, start, end, err)
				// If server ignored range and returned 200 while start>0, downloadChunk returns error; decide strategy:
				// here we fail the attempt and on repeated failures we abort
				continue
			}
			success = true
			break
		}
		if !success {
			return fmt.Errorf("chunk download failed after %d attempts: %v", MaxRetries, lastErr)
		}

		// update offset
		offset = dm.getLastOffset(filename)

		// If totalSize is unknown and a chunk returned with fewer bytes than requested and server closed, treat as done.
		if totalSize <= 0 && offset > 0 && offset < start+ChunkSize {
			// assumed EOF
			break
		}

		// If we know totalSize and we've reached it, break
		if totalSize > 0 && offset >= totalSize {
			break
		}
	}

	// At this point, tmp file should be complete. Verify MD5 (if provided) and promote.
	tmpPath := filepath.Join(DownloadsDir, filename+tmpSuffix)
	// fsync already done in downloadChunk per-chunk close; ensure final fsync by opening and Sync
	if f, err := os.Open(tmpPath); err == nil {
		_ = f.Sync()
		_ = f.Close()
	}

	if err := dm.verifyMD5(tmpPath, expectedMD5); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("md5 verification failed: %w", err)
	}

	finalPath := filepath.Join(DownloadsDir, filename)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename tmp->final failed: %w", err)
	}

	log.Printf("[DOWNLOAD] saved to %s", finalPath)
	return nil
}

// safeRedirectChecker enforces https and allowed host suffixes and optional md5 consistency
func safeRedirectChecker(allowedSuffixes []string) func(req *http.Request, via []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" {
			return fmt.Errorf("redirect to non-https blocked: %s", req.URL.String())
		}
		if len(allowedSuffixes) > 0 {
			ok := false
			for _, suf := range allowedSuffixes {
				if strings.HasSuffix(req.URL.Host, suf) {
					ok = true
					break
				}
			}
			if !ok {
				return fmt.Errorf("redirect to disallowed host: %s", req.URL.Host)
			}
		}
		// preserve md5 query param across redirects if present on original request
		if len(via) > 0 {
			orig := via[0].URL.Query().Get("md5")
			next := req.URL.Query().Get("md5")
			if orig != "" && next != "" && orig != next {
				return fmt.Errorf("md5 mismatch on redirect")
			}
		}
		return nil
	}
}

var safeNameRe = regexp.MustCompile(`[^A-Za-z0-9\-\._]`)

func sanitizeFilename(name string) string {
	if name == "" {
		name = fmt.Sprintf("download_%d", time.Now().Unix())
	}
	name = filepath.Base(name) // strip path
	name = safeNameRe.ReplaceAllString(name, "_")
	if len(name) > 200 {
		name = name[:200]
	}
	return name
}

// getFileSize performs a HEAD via Tor to learn content length when available.
func (dm *DownloadManager) getFileSize(rawurl string) (int64, error) {
	req, err := http.NewRequest("HEAD", rawurl, nil)
	if err != nil {
		return 0, err
	}
	resp, err := dm.hc.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return 0, fmt.Errorf("HEAD request failed with status: %d", resp.StatusCode)
	}
	cl := resp.Header.Get("Content-Length")
	if cl == "" {
		return 0, fmt.Errorf("Content-Length header missing")
	}
	n, err := strconv.ParseInt(cl, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid Content-Length: %s", cl)
	}
	return n, nil
}

// getLastOffset checks the .tmp file size so resume starts where it left off.
func (dm *DownloadManager) getLastOffset(filename string) int64 {
	tmpPath := filepath.Join(DownloadsDir, filename+tmpSuffix)
	info, err := os.Stat(tmpPath)
	if err != nil {
		return 0
	}
	return info.Size()
}

// downloadChunk writes the requested range into filename+tmpSuffix at the correct offset.
// start: inclusive start byte; end: inclusive end byte (0 => until EOF).
func (dm *DownloadManager) downloadChunk(rawurl string, start, end int64, filename string) error {
	req, err := http.NewRequest("GET", rawurl, nil)
	if err != nil {
		return err
	}
	if end > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	} else {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", start))
	}

	resp, err := dm.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Handle response codes: 206 preferred for range, 200 only allowed if start==0
	if resp.StatusCode == http.StatusPartialContent {
		// Validate Content-Range header contains expected start
		cr := resp.Header.Get("Content-Range") // e.g. "bytes 100-199/1000"
		if cr == "" {
			return fmt.Errorf("206 response missing Content-Range")
		}
		var crStart int64
		_, scanErr := fmt.Sscanf(cr, "bytes %d-", &crStart)
		if scanErr != nil {
			// Try alternative parsing
			var a, b, c int64
			if _, err := fmt.Sscanf(cr, "bytes %d-%d/%d", &a, &b, &c); err == nil {
				crStart = a
			}
		}
		if crStart != start {
			return fmt.Errorf("Content-Range start mismatch: expected %d got %d (header=%s)", start, crStart, cr)
		}
	} else if resp.StatusCode == http.StatusOK {
		if start != 0 {
			return fmt.Errorf("server ignored Range (200 OK) while resuming at %d; refusing to append", start)
		}
	} else {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Prepare tmp file and write at offset (no O_APPEND).
	tmpPath := filepath.Join(DownloadsDir, filename+tmpSuffix)
	if err := os.MkdirAll(DownloadsDir, 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open temp file: %w", err)
	}
	// Seek to expected start
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		f.Close()
		return fmt.Errorf("seek failed: %w", err)
	}

	// If end specified, copy exactly that many bytes; else copy until EOF.
	var toCopy int64 = -1
	if end > 0 {
		toCopy = end - start + 1
	}

	buf := make([]byte, 32*1024)
	var written int64
	for {
		// Determine read size for this iteration
		readSize := len(buf)
		if toCopy >= 0 {
			remaining := toCopy - written
			if remaining <= 0 {
				break
			}
			if int64(readSize) > remaining {
				readSize = int(remaining)
			}
		}
		n, rErr := resp.Body.Read(buf[:readSize])
		if n > 0 {
			wn, wErr := f.Write(buf[:n])
			if wErr != nil {
				f.Close()
				return fmt.Errorf("write error: %w", wErr)
			}
			if wn != n {
				f.Close()
				return fmt.Errorf("short write: %d != %d", wn, n)
			}
			written += int64(n)
		}
		if rErr == io.EOF {
			break
		}
		if rErr != nil {
			f.Close()
			return fmt.Errorf("read error: %w", rErr)
		}
	}

	// flush to disk
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("fsync failed: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close failed: %w", err)
	}

	// basic sanity check
	info, err := os.Stat(tmpPath)
	if err != nil {
		return fmt.Errorf("stat tmp failed: %w", err)
	}
	if info.Size() < start+written {
		return fmt.Errorf("tmp file shorter than expected: %d < %d", info.Size(), start+written)
	}

	return nil
}

// verifyMD5 verifies the downloaded file's MD5 checksum (path should point to tmp or final)
func (dm *DownloadManager) verifyMD5(path, expectedMD5 string) error {
	if expectedMD5 == "" {
		log.Printf("[DOWNLOAD] No MD5 provided for %s", path)
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, expectedMD5) {
		return fmt.Errorf("md5 mismatch: expected %s got %s", expectedMD5, got)
	}
	return nil
}
