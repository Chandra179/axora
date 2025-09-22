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
}

// NewCore creates a new Core instance with the required dependencies
func NewCore(pdfExtractor, epubExtractor TextExtractor, directoryPath string) *Core {
	return &Core{
		pdfExtractor:  pdfExtractor,
		epubExtractor: epubExtractor,
		directoryPath: directoryPath,
	}
}

// processFile processes a single file and returns the result
func (c *Core) processFile(path string, fileInfo os.FileInfo) *FileResult {
	fileSize := fileInfo.Size()
	extension := strings.ToLower(filepath.Ext(path))
	fileName := fileInfo.Name()

	fmt.Printf("Processing file: %s (Size: %d bytes, Extension: %s)\n", path, fileSize, extension)

	var extractedText string

	switch extension {
	case ".pdf":
		if c.pdfExtractor != nil {
			extractedText = c.pdfExtractor.ExtractText(path)
		}
	case ".epub":
		if c.epubExtractor != nil {
			extractedText = c.epubExtractor.ExtractText(path)
		}
	default:
		fmt.Printf("Unsupported file type: %s\n", extension)
		return nil
	}

	// Filter out failed extractions
	if extractedText == "" {
		return nil
	}

	return &FileResult{
		FileName:      fileName,
		FileSize:      fileSize,
		FileExtension: extension,
		ExtractedText: extractedText,
		TextLength:    len(extractedText),
	}
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
