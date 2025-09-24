package file

import (
	"bytes"
	"image/png"

	"github.com/gen2brain/go-fitz"
	"github.com/otiai10/gosseract/v2"
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

func (p *PDFExtractor) ExtractText(fp string) {
	doc, err := fitz.New(fp)
	if err != nil {
		p.logger.Error("Failed to open PDF for OCR", zap.String("file", fp), zap.Error(err))
		return
	}
	defer doc.Close()

	client := gosseract.NewClient()
	defer client.Close()

	// Configure Tesseract for better OCR accuracy
	client.SetVariable("tessedit_ocr_engine_mode", "1")  // LSTM only
	client.SetVariable("tessedit_pageseg_mode", "3")     // Fully automatic page segmentation
	client.SetVariable("tessedit_char_blacklist", "")    // Remove any character restrictions
	client.SetVariable("preserve_interword_spaces", "1") // Preserve spacing

	for pageNum := 0; pageNum < doc.NumPage(); pageNum++ {
		// Extract at high DPI (600 is good, you already have this)
		img, err := doc.ImageDPI(pageNum, 600)
		if err != nil {
			p.logger.Error("Failed to convert page to image",
				zap.String("file", fp),
				zap.Int("page", pageNum+1),
				zap.Error(err))
			continue
		}

		var buf bytes.Buffer
		encoder := png.Encoder{
			CompressionLevel: png.NoCompression, // Use no compression for better quality
		}
		if err := encoder.Encode(&buf, img); err != nil {
			p.logger.Error("Failed to encode PNG", zap.Error(err))
			continue
		}

		if err := client.SetImageFromBytes(buf.Bytes()); err != nil {
			p.logger.Error("Failed to set image for OCR", zap.Error(err))
			continue
		}

		text, err := client.Text()
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
	}
}
