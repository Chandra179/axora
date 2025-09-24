package file

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.uber.org/zap"
)

type Core struct {
	pdfExtractor  TextExtractor
	epubExtractor TextExtractor
	directoryPath string
	logger        *zap.Logger
}

type FileResult struct {
	FileName      string `json:"fileName"`
	FileSize      int64  `json:"fileSize"`
	FileExtension string `json:"fileExtension"`
	ExtractedText string `json:"extractedText"`
	TextLength    int    `json:"textLength"`
	Pages         int    `json:"pages,omitempty"`    // For PDFs
	Chapters      int    `json:"chapters,omitempty"` // For EPUBs
	ProcessTime   string `json:"processTime,omitempty"`
	Error         string `json:"error,omitempty"`
}

func NewCore(pdfExtractor, epubExtractor TextExtractor, directoryPath string, logger *zap.Logger) *Core {
	return &Core{
		pdfExtractor:  pdfExtractor,
		epubExtractor: epubExtractor,
		directoryPath: directoryPath,
		logger:        logger,
	}
}

// processFile processes a single file and returns the result with proper error handling
func (c *Core) processFile(path string, fileInfo os.FileInfo) *FileResult {
	fileSize := fileInfo.Size()
	extension := strings.ToLower(filepath.Ext(path))
	fileName := fileInfo.Name()

	c.logger.Info("Processing file",
		zap.String("path", path),
		zap.Int64("size", fileSize),
		zap.String("extension", extension))

	result := &FileResult{
		FileName:      fileName,
		FileSize:      fileSize,
		FileExtension: extension,
	}

	// Add recovery mechanism to handle panics
	defer func() {
		if r := recover(); r != nil {
			result.Error = fmt.Sprintf("Panic during processing: %v", r)
			c.logger.Error("Recovered from panic while processing file",
				zap.String("path", path),
				zap.Any("panic", r))
		}
	}()

	var extractionResult *ExtractionResult

	switch extension {
	case ".pdf":
		if c.pdfExtractor != nil {
			extractionResult = c.pdfExtractor.ExtractText(path)
		}
	case ".epub":
		if c.epubExtractor != nil {
			extractionResult = c.epubExtractor.ExtractText(path)
		}
	default:
		result.Error = fmt.Sprintf("Unsupported file type: %s", extension)
		c.logger.Warn("Unsupported file type",
			zap.String("path", path),
			zap.String("extension", extension))
		return result
	}

	if extractionResult == nil {
		result.Error = "No extractor available for this file type"
		return result
	}

	if !extractionResult.Success {
		result.Error = extractionResult.Error
		c.logger.Error("Text extraction failed",
			zap.String("path", path),
			zap.String("error", extractionResult.Error))
		return result
	}

	if extractionResult.Text == "" {
		result.Error = "No text extracted"
		c.logger.Warn("No text content extracted",
			zap.String("path", path))
		return result
	}

	result.ExtractedText = extractionResult.Text
	result.TextLength = len(extractionResult.Text)
	result.Pages = extractionResult.Pages
	result.Chapters = extractionResult.Chapters

	c.logger.Info("File processed successfully",
		zap.String("path", path),
		zap.Int("textLength", result.TextLength),
		zap.Int("pages", result.Pages),
		zap.Int("chapters", result.Chapters))

	return result
}

// ProcessFiles processes all supported files in the directory and returns structured results
func (c *Core) ProcessFiles() []FileResult {
	var wg sync.WaitGroup
	resultChan := make(chan *FileResult, 100)
	var results []FileResult

	c.logger.Info("Starting batch file processing",
		zap.String("directory", c.directoryPath))

	// Start a goroutine to collect results
	done := make(chan bool)
	go func() {
		for result := range resultChan {
			if result != nil {
				results = append(results, *result)
			}
		}
		done <- true
	}()

	err := filepath.WalkDir(c.directoryPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			c.logger.Error("Error walking directory",
				zap.String("path", path),
				zap.Error(err))
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		fileInfo, err := os.Stat(path)
		if err != nil {
			c.logger.Error("Error getting file info",
				zap.String("path", path),
				zap.Error(err))
			return nil
		}

		extension := strings.ToLower(filepath.Ext(path))
		if extension != ".pdf" && extension != ".epub" {
			return nil
		}

		const maxFileSize = 50 * 1024 * 1024
		if fileInfo.Size() > maxFileSize {
			c.logger.Warn("Skipping large file",
				zap.String("path", path),
				zap.Int64("size", fileInfo.Size()),
				zap.Int64("maxSize", maxFileSize))
			return nil
		}

		wg.Add(1)
		go func(filePath string, info os.FileInfo) {
			defer wg.Done()
			result := c.processFile(filePath, info)
			resultChan <- result
		}(path, fileInfo)

		return nil
	})

	if err != nil {
		c.logger.Error("Error walking directory",
			zap.String("directory", c.directoryPath),
			zap.Error(err))
		return []FileResult{}
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(resultChan)

	// Wait for result collection to complete
	<-done

	successful := c.GetSuccessfulResults(results)
	failed := c.GetFailedResults(results)

	c.logger.Info("Batch processing completed",
		zap.String("directory", c.directoryPath),
		zap.Int("totalFiles", len(results)),
		zap.Int("successful", len(successful)),
		zap.Int("failed", len(failed)))

	return results
}

// GetSuccessfulResults returns only results that were processed successfully
func (c *Core) GetSuccessfulResults(results []FileResult) []FileResult {
	var successful []FileResult
	for _, result := range results {
		if result.Error == "" && result.ExtractedText != "" {
			successful = append(successful, result)
		}
	}
	return successful
}

// GetFailedResults returns only results that failed processing
func (c *Core) GetFailedResults(results []FileResult) []FileResult {
	var failed []FileResult
	for _, result := range results {
		if result.Error != "" {
			failed = append(failed, result)
		}
	}
	return failed
}

// GetProcessingStats returns statistics about the processing results
func (c *Core) GetProcessingStats(results []FileResult) map[string]interface{} {
	successful := c.GetSuccessfulResults(results)
	failed := c.GetFailedResults(results)

	totalSize := int64(0)
	totalTextLength := 0
	totalPages := 0
	totalChapters := 0

	for _, result := range successful {
		totalSize += result.FileSize
		totalTextLength += result.TextLength
		totalPages += result.Pages
		totalChapters += result.Chapters
	}

	stats := map[string]interface{}{
		"totalFiles":      len(results),
		"successfulFiles": len(successful),
		"failedFiles":     len(failed),
		"successRate":     float64(len(successful)) / float64(len(results)) * 100,
		"totalFileSize":   totalSize,
		"totalTextLength": totalTextLength,
		"totalPages":      totalPages,
		"totalChapters":   totalChapters,
	}

	c.logger.Info("Processing statistics",
		zap.Any("stats", stats))

	return stats
}
