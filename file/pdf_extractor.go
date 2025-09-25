package file

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gen2brain/go-fitz"
	"github.com/otiai10/gosseract/v2"
	"go.uber.org/zap"
)

type PDFExtractor struct {
	logger          *zap.Logger
	gosseractClient *gosseract.Client
}

func NewPDFExtractor(logger *zap.Logger) *PDFExtractor {
	client := gosseract.NewClient()
	client.SetLanguage("eng")
	client.SetVariable("tessedit_ocr_engine_mode", "1")
	client.SetVariable("tessedit_pageseg_mode", "6")

	// Noise control
	client.SetVariable("textord_noise_normratio", "0.5")
	client.SetVariable("textord_noise_sizelimit", "0.3")
	client.SetVariable("textord_heavy_nr", "1")

	// Character control
	client.SetVariable("classify_enable_learning", "0") // deterministic
	client.SetVariable("tessedit_write_block_separators", "0")

	// Reject garbage
	client.SetVariable("tessedit_reject_block_percent", "80")
	client.SetVariable("tessedit_minimal_rejection", "1")
	client.SetVariable("tessedit_reject_mode", "0")

	// Misc
	client.SetVariable("tessedit_do_invert", "0")
	client.SetVariable("textord_tablefind_recognize_tables", "0")

	return &PDFExtractor{
		logger:          logger,
		gosseractClient: client,
	}
}

func (p *PDFExtractor) ExtractText(fp string) {
	doc, err := fitz.New(fp)
	if err != nil {
		p.logger.Error("Failed to open PDF for OCR", zap.String("file", fp), zap.Error(err))
		return
	}
	defer doc.Close()
	totalPages := doc.NumPage()

	for pageNum := 0; pageNum < totalPages; pageNum++ {
		img, err := doc.ImageDPI(pageNum, 300)
		if err != nil {
			p.logger.Error("Failed to convert page to image",
				zap.String("file", fp),
				zap.Int("page", pageNum+1),
				zap.Error(err))
			continue
		}

		grayImg := p.convertToGrayscale(img)
		processedImg := p.enhanceContrast(grayImg)

		var buf bytes.Buffer
		encoder := png.Encoder{CompressionLevel: png.DefaultCompression}
		if err := encoder.Encode(&buf, processedImg); err != nil {
			p.logger.Error("Failed to encode PNG",
				zap.String("file", fp),
				zap.Int("page", pageNum+1),
				zap.Error(err))
			continue
		}

		if pageNum < 3 {
			if err := p.savePNGToDisk(buf.Bytes(), fp, pageNum); err != nil {
				p.logger.Error("Failed to save PNG to disk",
					zap.String("file", fp),
					zap.Int("page", pageNum+1),
					zap.Error(err))
			}
		}

		if err := p.gosseractClient.SetImageFromBytes(buf.Bytes()); err != nil {
			p.logger.Error("Failed to set image for OCR",
				zap.String("file", fp),
				zap.Int("page", pageNum+1),
				zap.Error(err))
			continue
		}
		buf.Reset()

		text, err := p.gosseractClient.Text()
		if err != nil {
			p.logger.Error("Failed to extract text via OCR",
				zap.String("file", fp),
				zap.Int("page", pageNum+1),
				zap.Error(err))
		} else {
			cleanTxt := cleanOCR(text)
			p.logger.Info("OCR result",
				zap.String("file", fp),
				zap.Int("page", pageNum+1),
				zap.String("text", cleanTxt))
		}

		img = nil
		grayImg = nil
		processedImg = nil
	}
}

func (p *PDFExtractor) savePNGToDisk(pngData []byte, originalFilePath string, pageNum int) error {
	outputDir := filepath.Join("/app/downloads/temp")

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	outputPath := filepath.Join(outputDir, fmt.Sprintf("page_%03d.png", pageNum+1))

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create PNG file: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(pngData); err != nil {
		return fmt.Errorf("failed to write PNG data: %w", err)
	}

	p.logger.Info("Saved processed PNG",
		zap.String("output", outputPath),
		zap.Int("page", pageNum+1),
		zap.Int("size_bytes", len(pngData)))

	return nil
}

// Convert image to grayscale for better OCR performance
func (p *PDFExtractor) convertToGrayscale(src image.Image) image.Image {
	bounds := src.Bounds()
	gray := image.NewGray(bounds)
	draw.Draw(gray, bounds, src, bounds.Min, draw.Src)
	return gray
}

// Enhance contrast for better OCR accuracy
func (p *PDFExtractor) enhanceContrast(src image.Image) image.Image {
	bounds := src.Bounds()
	enhanced := image.NewGray(bounds)

	// Calculate histogram for adaptive enhancement
	histogram := make([]int, 256)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			grayPixel := src.(*image.Gray).GrayAt(x, y)
			histogram[grayPixel.Y]++
		}
	}

	// Find min and max non-zero values for contrast stretching
	var min, max uint8 = 255, 0
	totalPixels := (bounds.Max.X - bounds.Min.X) * (bounds.Max.Y - bounds.Min.Y)

	for i := 0; i < 256; i++ {
		if histogram[i] > totalPixels/1000 { // Ignore noise (< 0.1% of pixels)
			if uint8(i) < min {
				min = uint8(i)
			}
			if uint8(i) > max {
				max = uint8(i)
			}
		}
	}

	// Apply contrast stretching
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			oldPixel := src.(*image.Gray).GrayAt(x, y)

			if max > min {
				// Linear contrast stretching
				newValue := uint8(float64(oldPixel.Y-min) * 255.0 / float64(max-min))
				enhanced.Set(x, y, color.Gray{Y: newValue})
			} else {
				enhanced.Set(x, y, oldPixel)
			}
		}
	}

	return enhanced
}

func cleanOCR(input string) string {
	text := input

	// 1. Fix hyphenated line breaks: "gov-\nernment" → "government"
	reHyphen := regexp.MustCompile(`(\w)-\n(\w)`)
	text = reHyphen.ReplaceAllString(text, "$1$2")

	// 2. Replace all newlines with spaces (flatten for embeddings)
	text = strings.ReplaceAll(text, "\n", " ")

	// 3. Remove headers/footers: ALL CAPS + page numbers
	reHeader := regexp.MustCompile(`\b[A-Z\s]{3,}\s*\d+\b`)
	text = reHeader.ReplaceAllString(text, " ")

	// 4. Remove figure/table references
	reFigure := regexp.MustCompile(`\b(Figure|Table)\s*\d+[-–]?\d*\b`)
	text = reFigure.ReplaceAllString(text, " ")

	// 5. Remove margin notes like QUICK QUIZ, APPENDIX, CASE STUDY
	reMargin := regexp.MustCompile(`\b(QUICK QUIZ|SUMMARY|APPENDIX|CASE STUDY)\b.*`)
	text = reMargin.ReplaceAllString(text, " ")

	// 8. Remove OCR artifacts (®, ©, ™, bullets, etc.)
	reArtifacts := regexp.MustCompile(`[®©™•▪●►■□▪¤]+`)
	text = reArtifacts.ReplaceAllString(text, " ")

	// 9. Remove chart/axis junk: lines of mostly | / \ - + = ~
	reAxis := regexp.MustCompile(`[|/\-+=~]{2,}`)
	text = reAxis.ReplaceAllString(text, " ")

	// 11. Trim
	text = strings.TrimSpace(text)

	return text
}
