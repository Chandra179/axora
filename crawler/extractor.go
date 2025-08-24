package crawler

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/go-shiori/go-readability"
)

type ContentExtractor struct{}

func NewContentExtractor() *ContentExtractor {
	return &ContentExtractor{}
}

func (ce *ContentExtractor) ExtractText(htmlContent string, url *url.URL) (string, error) {
	fmt.Println("Starting text extraction with go-readability")
	article, err := readability.FromReader(strings.NewReader(htmlContent), url)
	if err != nil {
		fmt.Printf("Error processing with readability: %v\n", err)
		return "", err
	}

	extractedText := strings.TrimSpace(article.TextContent)
	extractedText = strings.Join(strings.Fields(extractedText), " ")

	fmt.Printf("Extracted text length: %d characters\n", len(extractedText))
	return extractedText, nil
}
