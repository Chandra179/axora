package file

import (
	"bytes"
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
	client.SetVariable("tessedit_pageseg_mode", "3")
	client.SetVariable("tessedit_char_blacklist", "")

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
		img, err := doc.ImageDPI(pageNum, 600)
		if err != nil {
			p.logger.Error("Failed to convert page to image",
				zap.String("file", fp),
				zap.Int("page", pageNum+1),
				zap.Error(err))
			continue
		}

		var buf bytes.Buffer
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		if err := encoder.Encode(&buf, img); err != nil {
			p.logger.Error("Failed to encode PNG",
				zap.String("file", fp),
				zap.Int("page", pageNum+1),
				zap.Error(err))
			continue
		}

		// Free memory early
		img = nil

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
	}
}
