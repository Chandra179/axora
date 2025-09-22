package file

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Core implements the TextExtractor interface
type Core struct {
	pdfExtractor  TextExtractor
	epubExtractor TextExtractor
	directoryPath string
}

// FileResult holds the result of processing a single file
type FileResult struct {
	FileName      string
	FileSize      int64
	FileExtension string
	ExtractedText string
	TextLength    int
	Error         string // Add error field to track processing issues
}

// NewCore creates a new Core instance with the required dependencies
func NewCore(pdfExtractor, epubExtractor TextExtractor, directoryPath string) *Core {
	return &Core{
		pdfExtractor:  pdfExtractor,
		epubExtractor: epubExtractor,
		directoryPath: directoryPath,
	}
}

// processFile processes a single file and returns the result with proper error handling
func (c *Core) processFile(path string, fileInfo os.FileInfo) *FileResult {
	fileSize := fileInfo.Size()
	extension := strings.ToLower(filepath.Ext(path))
	fileName := fileInfo.Name()

	fmt.Printf("Processing file: %s (Size: %d bytes, Extension: %s)\n", path, fileSize, extension)

	result := &FileResult{
		FileName:      fileName,
		FileSize:      fileSize,
		FileExtension: extension,
	}

	// Add recovery mechanism to handle panics
	defer func() {
		if r := recover(); r != nil {
			result.Error = fmt.Sprintf("Panic during processing: %v", r)
			fmt.Printf("Recovered from panic while processing %s: %v\n", path, r)
		}
	}()

	var extractedText string
	var err error

	switch extension {
	case ".pdf":
		if c.pdfExtractor != nil {
			// Wrap the extraction in a safe call
			extractedText, err = c.safeExtractText(c.pdfExtractor, path)
			if err != nil {
				result.Error = fmt.Sprintf("PDF extraction error: %v", err)
				fmt.Printf("Error extracting PDF %s: %v\n", path, err)
				return result
			}
		}
	case ".epub":
		if c.epubExtractor != nil {
			// Wrap the extraction in a safe call
			extractedText, err = c.safeExtractText(c.epubExtractor, path)
			if err != nil {
				result.Error = fmt.Sprintf("EPUB extraction error: %v", err)
				fmt.Printf("Error extracting EPUB %s: %v\n", path, err)
				return result
			}
		}
	default:
		result.Error = fmt.Sprintf("Unsupported file type: %s", extension)
		fmt.Printf("Unsupported file type: %s\n", extension)
		return result
	}

	// Filter out failed extractions
	if extractedText == "" {
		result.Error = "No text extracted"
		return result
	}

	result.ExtractedText = extractedText
	result.TextLength = len(extractedText)

	return result
}

// safeExtractText wraps the text extraction with panic recovery
func (c *Core) safeExtractText(extractor TextExtractor, path string) (text string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("extraction panic: %v", r)
		}
	}()

	text = extractor.ExtractText(path)
	return text, nil
}

// ProcessFiles processes all supported files in the directory and returns structured results
func (c *Core) ProcessFiles() []FileResult {
	var wg sync.WaitGroup
	resultChan := make(chan *FileResult, 100) // Buffered channel to prevent blocking
	var results []FileResult

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
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Get file info to check size
		fileInfo, err := os.Stat(path)
		if err != nil {
			fmt.Printf("Error getting file info for %s: %v\n", path, err)
			return nil // Continue processing other files
		}

		// Only process supported file types
		extension := strings.ToLower(filepath.Ext(path))
		if extension != ".pdf" && extension != ".epub" {
			return nil
		}

		// Add file size check to skip potentially problematic very large files
		const maxFileSize = 50 * 1024 * 1024 // 50MB limit
		if fileInfo.Size() > maxFileSize {
			fmt.Printf("Skipping large file %s (size: %d bytes)\n", path, fileInfo.Size())
			return nil
		}

		// Launch a goroutine for each supported file
		wg.Add(1)
		go func(filePath string, info os.FileInfo) {
			defer wg.Done()
			result := c.processFile(filePath, info)
			resultChan <- result
		}(path, fileInfo)

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking directory: %v\n", err)
		return []FileResult{}
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(resultChan)

	// Wait for result collection to complete
	<-done

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
