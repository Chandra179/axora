package file

import (
	"bytes"
	"image/png"
	"runtime"

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

	// Process pages in batches to manage memory
	batchSize := 5 // Process 5 pages at a time
	totalPages := doc.NumPage()

	for startPage := 0; startPage < totalPages; startPage += batchSize {
		endPage := startPage + batchSize
		if endPage > totalPages {
			endPage = totalPages
		}

		p.logger.Info("Processing page batch",
			zap.String("file", fp),
			zap.Int("start_page", startPage+1),
			zap.Int("end_page", endPage),
			zap.Int("total_pages", totalPages))

		p.processBatch(doc, fp, startPage, endPage)

		// Force garbage collection between batches
		runtime.GC()
	}
}

func (p *PDFExtractor) processBatch(doc *fitz.Document, fp string, startPage, endPage int) {
	// Create a new Tesseract client for each batch
	client := gosseract.NewClient()
	defer client.Close()

	// Configure Tesseract for better OCR accuracy
	client.SetVariable("tessedit_ocr_engine_mode", "1")  // LSTM only
	client.SetVariable("tessedit_pageseg_mode", "3")     // Fully automatic page segmentation
	client.SetVariable("tessedit_char_blacklist", "")    // Remove any character restrictions
	client.SetVariable("preserve_interword_spaces", "1") // Preserve spacing

	for pageNum := startPage; pageNum < endPage; pageNum++ {
		p.processPage(client, doc, fp, pageNum)
	}
}

func (p *PDFExtractor) processPage(client *gosseract.Client, doc *fitz.Document, fp string, pageNum int) {
	// Reduce DPI if memory is an issue (300 is often sufficient for OCR)
	img, err := doc.ImageDPI(pageNum, 600) // Reduced from 600 to 300
	if err != nil {
		p.logger.Error("Failed to convert page to image",
			zap.String("file", fp),
			zap.Int("page", pageNum+1),
			zap.Error(err))
		return
	}

	// Use a local buffer that will be garbage collected after this function
	var buf bytes.Buffer
	encoder := png.Encoder{
		CompressionLevel: png.BestSpeed, // Faster encoding, less memory usage
	}

	if err := encoder.Encode(&buf, img); err != nil {
		p.logger.Error("Failed to encode PNG",
			zap.String("file", fp),
			zap.Int("page", pageNum+1),
			zap.Error(err))
		return
	}

	// Clear the image reference immediately
	img = nil

	if err := client.SetImageFromBytes(buf.Bytes()); err != nil {
		p.logger.Error("Failed to set image for OCR",
			zap.String("file", fp),
			zap.Int("page", pageNum+1),
			zap.Error(err))
		return
	}

	// Clear the buffer immediately after use
	buf.Reset()

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
