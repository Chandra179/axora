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
		p.logger.Error("Failed to open PDF", zap.String("file", fp), zap.Error(err))
		return
	}
	defer doc.Close()

	client := gosseract.NewClient()
	defer client.Close()

	for pageNum := 0; pageNum < doc.NumPage(); pageNum++ {
		p.logger.Info("Processing page", zap.String("file", fp), zap.Int("page", pageNum+1))

		// Convert page to image
		img, err := doc.Image(pageNum)
		if err != nil {
			p.logger.Error("Failed to convert page to image",
				zap.String("file", fp),
				zap.Int("page", pageNum+1),
				zap.Error(err))
			continue
		}

		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
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
