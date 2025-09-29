package text

import (
	"go.uber.org/zap"
)

type EpubExtractor struct {
	logger *zap.Logger
}

func NewEpubExtractor(logger *zap.Logger) *EpubExtractor {
	return &EpubExtractor{
		logger: logger,
	}
}

func (p *EpubExtractor) ExtractText(fp string) {}
