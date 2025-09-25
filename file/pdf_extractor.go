package file

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"

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
	client.SetVariable("tessedit_ocr_engine_mode", "1")
	client.SetVariable("tessedit_pageseg_mode", "1")
	client.SetVariable("tessedit_char_blacklist", "")
	client.SetVariable("tessedit_do_invert", "0")
	client.SetVariable("classify_enable_learning", "0")
	client.SetVariable("textord_noise_normratio", "2")
	client.SetVariable("textord_noise_sizelimit", "0.5")

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
			p.logger.Info("OCR result",
				zap.String("file", fp),
				zap.Int("page", pageNum+1),
				zap.String("text", text))
		}

		img = nil
		grayImg = nil
		processedImg = nil
	}
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
