package file

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Core implements the TextExtractor interface
type Core struct {
	pdfExtractor  TextExtractor
	epubExtractor TextExtractor
	directoryPath string
}

// NewCore creates a new Core instance with the required dependencies
func NewCore(pdfExtractor, epubExtractor TextExtractor, directoryPath string) *Core {
	return &Core{
		pdfExtractor:  pdfExtractor,
		epubExtractor: epubExtractor,
		directoryPath: directoryPath,
	}
}

// ProcessFiles processes all supported files in the directory and extracts text
func (c *Core) ProcessFiles() string {
	var result strings.Builder

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

		fileSize := fileInfo.Size()
		extension := strings.ToLower(filepath.Ext(path))

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

		if extractedText != "" {
			result.WriteString(fmt.Sprintf("\n=== Text from %s ===\n", path))
			result.WriteString(extractedText)
			result.WriteString("\n")
		}

		return nil
	})

	if err != nil {
		return fmt.Sprintf("Error walking directory: %v", err)
	}

	return result.String()
}
