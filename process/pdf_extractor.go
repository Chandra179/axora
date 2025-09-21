package processor

import (
	"fmt"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

// LedongthucExtractor implements PDFExtractor using github.com/ledongthuc/pdf
type LedongthucExtractor struct{}

// NewLedongthucExtractor creates a new instance of LedongthucExtractor
func NewLedongthucExtractor() *LedongthucExtractor {
	return &LedongthucExtractor{}
}

// ExtractFromFile extracts text from a PDF file path
func (e *LedongthucExtractor) ExtractFromFile(filePath string) (string, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open PDF file: %w", err)
	}
	defer f.Close()

	return e.extractText(r)
}

// extractText extracts text from a PDF reader
func (e *LedongthucExtractor) extractText(r *pdf.Reader) (string, error) {
	reader, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("failed to extract plain text: %w", err)
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read plain text: %w", err)
	}

	return strings.TrimSpace(string(content)), nil
}
