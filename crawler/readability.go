package crawler

import (
	"net/url"
	"strings"

	"github.com/go-shiori/go-readability"
)

type ReadabilityExtractor struct{}

func NewReadabilityExtractor() *ReadabilityExtractor {
	return &ReadabilityExtractor{}
}

func (re *ReadabilityExtractor) ExtractText(htmlContent, urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	article, err := readability.FromReader(strings.NewReader(htmlContent), parsedURL)
	if err != nil {
		return "", err
	}

	return article.TextContent, nil
}
