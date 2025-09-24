package file

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/ledongthuc/pdf"
	"go.uber.org/zap"
)

type PDFExtractor struct {
	logger *zap.Logger
}

func NewPDFExtractor(logger *zap.Logger) *PDFExtractor {
	return &PDFExtractor{
		logger: logger,
	}
}

func (p *PDFExtractor) ExtractText(filepath string) *ExtractionResult {
	result := &ExtractionResult{
		FilePath: filepath,
		Success:  false,
	}

	if err := p.validateFile(filepath); err != nil {
		result.Error = fmt.Sprintf("File validation failed: %v", err)
		p.logger.Error("PDF file validation failed",
			zap.String("filepath", filepath),
			zap.Error(err))
		return result
	}

	f, r, err := pdf.Open(filepath)
	if err != nil {
		result.Error = fmt.Sprintf("Error opening PDF file: %v", err)
		p.logger.Error("Failed to open PDF file",
			zap.String("filepath", filepath),
			zap.Error(err))
		return result
	}
	defer f.Close()

	totalPages := r.NumPage()
	result.Pages = totalPages

	var textBuilder strings.Builder
	extractedPages := 0

	for i := 1; i <= totalPages; i++ {
		pageText, err := p.extractPageText(r, i)
		if err != nil {
			p.logger.Warn("Failed to extract text from page",
				zap.String("filepath", filepath),
				zap.Int("page", i),
				zap.Error(err))
			continue
		}

		if pageText != "" {
			textBuilder.WriteString(pageText)
			extractedPages++
		}
	}

	extractedText := strings.TrimSpace(textBuilder.String())
	extractedText = p.removePageNumbers(extractedText)
	extractedText = p.cleanText(extractedText)

	result.Text = extractedText
	result.Success = true

	p.logger.Info("Extracted text sample",
		zap.String("filepath", result.FilePath),
		zap.String("text_sample", extractedText[:1000]+"..."),
		zap.Int("full_text_length", len(result.Text)))
	return result
}

func (p *PDFExtractor) validateFile(filepath string) error {
	if filepath == "" {
		return fmt.Errorf("filepath cannot be empty")
	}

	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("cannot open file: %v", err)
	}
	defer file.Close()

	// Check if it's actually a PDF file (basic check)
	buffer := make([]byte, 4)
	_, err = file.Read(buffer)
	if err != nil {
		return fmt.Errorf("cannot read file header: %v", err)
	}

	if string(buffer) != "%PDF" {
		return fmt.Errorf("file does not appear to be a PDF")
	}

	return nil
}

func (p *PDFExtractor) extractPageText(reader *pdf.Reader, pageNum int) (string, error) {
	page := reader.Page(pageNum)
	if page.V.IsNull() {
		return "", fmt.Errorf("page %d is null", pageNum)
	}

	content, err := page.GetPlainText(nil)
	if err != nil {
		return "", fmt.Errorf("failed to get plain text: %v", err)
	}

	return strings.TrimSpace(content), nil
}

func (p *PDFExtractor) removePageNumbers(text string) string {
	// Common page number patterns
	patterns := []string{
		`(?m)^\s*\d+\s*$`,                   // Standalone numbers on their own line
		`(?m)^\s*Page\s+\d+\s*$`,            // "Page X" format
		`(?m)^\s*-\s*\d+\s*-\s*$`,           // "-X-" format
		`(?m)^\s*\d+\s*/\s*\d+\s*$`,         // "X/Y" format
		`(?m)^\s*\[\s*\d+\s*\]\s*$`,         // "[X]" format
		`(?m)^\s*Page\s+\d+\s+of\s+\d+\s*$`, // "Page X of Y" format
	}

	result := text
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		result = re.ReplaceAllString(result, "")
	}

	return result
}

func (p *PDFExtractor) cleanText(text string) string {
	re := regexp.MustCompile(`\n\s*\n\s*\n`)
	text = re.ReplaceAllString(text, "\n\n")

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}

	return strings.Join(lines, "\n")
}
