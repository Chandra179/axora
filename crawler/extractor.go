package crawler

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type ContentExtractor struct{}

func NewContentExtractor() *ContentExtractor {
	return &ContentExtractor{}
}

func (ce *ContentExtractor) ExtractText(htmlContent string) (string, error) {
	fmt.Println("Starting text extraction")
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		fmt.Printf("Error creating document: %v\n", err)
		return "", err
	}

	doc.Find("script, style, nav, header, footer, aside").Remove()
	fmt.Println("Removed unwanted elements")

	var texts []string

	doc.Find("main, article, section, p, h1, h2, h3, h4, h5, h6, li").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" && len(text) > 10 {
			texts = append(texts, text)
		}
	})
	fmt.Printf("Found %d main content texts\n", len(texts))

	if len(texts) == 0 {
		fmt.Println("No main content found, trying div elements")
		doc.Find("div").Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if text != "" && len(text) > 20 {
				texts = append(texts, text)
			}
		})
		fmt.Printf("Found %d div texts\n", len(texts))
	}

	fullText := strings.Join(texts, " ")
	fullText = strings.Join(strings.Fields(fullText), " ")
	fmt.Printf("Extracted text length: %d characters\n", len(fullText))

	return fullText, nil
}
