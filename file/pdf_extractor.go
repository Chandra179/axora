package file

import (
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

// PDFExtractor implements the TextExtractor interface for PDF files
type PDFExtractor struct{}

// NewPDFExtractor creates a new PDFExtractor instance
func NewPDFExtractor() *PDFExtractor {
	return &PDFExtractor{}
}

// ExtractText extracts text content from a PDF file
func (p *PDFExtractor) ExtractText(filepath string) string {
	// Open the PDF file
	f, r, err := pdf.Open(filepath)
	if err != nil {
		return fmt.Sprintf("Error opening PDF file %s: %v", filepath, err)
	}
	defer f.Close()

	var textBuilder strings.Builder
	totalPages := r.NumPage()

	// Extract text from each page
	for pageIndex := 1; pageIndex <= totalPages; pageIndex++ {
		page := r.Page(pageIndex)
		if page.V.IsNull() {
			continue
		}

		// Get text content from the page
		texts := page.Content().Text
		for _, text := range texts {
			textBuilder.WriteString(text.S)
			textBuilder.WriteString(" ")
		}
		textBuilder.WriteString("\n") // Add newline after each page
	}

	extractedText := textBuilder.String()
	if extractedText == "" {
		return fmt.Sprintf("No text content found in PDF file: %s", filepath)
	}

	return extractedText
}
