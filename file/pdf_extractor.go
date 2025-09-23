package file

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ledongthuc/pdf"
)

type PDFExtractor struct{}

func NewPDFExtractor() *PDFExtractor {
	return &PDFExtractor{}
}

func (p *PDFExtractor) validatePDFFile(filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("cannot open file: %v", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("cannot get file info: %v", err)
	}

	if fileInfo.Size() == 0 {
		return fmt.Errorf("file is empty")
	}

	// Check PDF header
	header := make([]byte, 4)
	_, err = file.Read(header)
	if err != nil {
		return fmt.Errorf("cannot read file header: %v", err)
	}

	if string(header) != "%PDF" {
		return fmt.Errorf("not a valid PDF file (missing PDF header)")
	}

	// Check for PDF trailer (EOF marker)
	// Seek to near the end of file to check for %%EOF
	seekPos := fileInfo.Size() - 1024
	if seekPos < 0 {
		seekPos = 0
	}

	_, err = file.Seek(seekPos, io.SeekStart)
	if err != nil {
		return fmt.Errorf("cannot seek in file: %v", err)
	}

	buffer := make([]byte, 1024)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("cannot read end of file: %v", err)
	}

	endContent := string(buffer[:n])
	if !strings.Contains(endContent, "%%EOF") {
		return fmt.Errorf("PDF file appears to be truncated (missing %%EOF)")
	}

	return nil
}

// ExtractText extracts text content from a PDF file with robust error handling
func (p *PDFExtractor) ExtractText(filepath string) string {
	if err := p.validatePDFFile(filepath); err != nil {
		return fmt.Sprintf("PDF validation failed for %s: %v", filepath, err)
	}

	// Use defer and recover to handle panics
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Recovered from panic while processing %s: %v\n", filepath, r)
		}
	}()

	f, r, err := pdf.Open(filepath)
	if err != nil {
		return fmt.Sprintf("Error opening PDF file %s: %v", filepath, err)
	}
	defer func() {
		if f != nil {
			f.Close()
		}
	}()

	var textBuilder strings.Builder
	totalPages := r.NumPage()

	// Extract text from each page with individual error handling
	for pageIndex := 1; pageIndex <= totalPages; pageIndex++ {
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Error processing page %d of %s: %v\n", pageIndex, filepath, r)
				}
			}()

			page := r.Page(pageIndex)
			if page.V.IsNull() {
				return
			}

			// Get text content from the page
			content := page.Content()
			texts := content.Text
			if len(texts) == 0 {
				return
			}

			// Process text elements
			for i, text := range texts {
				if text.S == "" {
					continue
				}

				textBuilder.WriteString(text.S)

				// Add space between text elements
				if i < len(texts)-1 && !strings.HasSuffix(text.S, " ") {
					nextText := texts[i+1].S
					if nextText != "" && !strings.HasPrefix(nextText, " ") {
						textBuilder.WriteString(" ")
					}
				}
			}

			textBuilder.WriteString("\n")
		}()
	}

	extractedText := strings.TrimSpace(textBuilder.String())
	if extractedText == "" {
		return fmt.Sprintf("No text content found in PDF file: %s", filepath)
	}

	return extractedText
}
