package processor

import "io"

// PDFExtractor defines the interface for PDF text extraction
type PDFExtractor interface {
	// ExtractFromFile extracts text from a PDF file path
	ExtractFromFile(filePath string) (string, error)

	// ExtractFromReader extracts text from an io.Reader
	ExtractFromReader(reader io.Reader) (string, error)
}

// Client wraps the PDFExtractor interface for easy swapping of implementations
type Client struct {
	extractor PDFExtractor
}

// NewClient creates a new PDF processor client with the given extractor implementation
func NewClient(extractor PDFExtractor) *Client {
	return &Client{
		extractor: extractor,
	}
}

// ExtractTextFromFile extracts text from a PDF file
func (c *Client) ExtractTextFromFile(filePath string) (string, error) {
	return c.extractor.ExtractFromFile(filePath)
}
