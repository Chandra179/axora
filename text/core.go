package text

import (
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

type Core struct {
	pdfExtractor  TextExtractor
	epubExtractor TextExtractor
	directoryPath string
	logger        *zap.Logger
}

func NewCore(pdfExtractor, epubExtractor TextExtractor, directoryPath string, logger *zap.Logger) *Core {
	return &Core{
		pdfExtractor:  pdfExtractor,
		epubExtractor: epubExtractor,
		directoryPath: directoryPath,
		logger:        logger,
	}
}

func (c *Core) processFile(path string) {
	extension := strings.ToLower(filepath.Ext(path))
	switch extension {
	case ".pdf":
		c.pdfExtractor.ExtractText(path)
	case ".epub":
		c.epubExtractor.ExtractText(path)
	default:
	}

}

func (c *Core) ProcessFiles() {
	err := filepath.Walk(c.directoryPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			c.logger.Error("Error walking directory", zap.Error(err))
			return err
		}

		if info.IsDir() {
			return nil
		}

		if strings.ToLower(filepath.Ext(path)) == ".pdf" {
			c.processFile(path)
		}

		return nil
	})

	if err != nil {
		c.logger.Error("Error processing files", zap.Error(err))
	}
}
