package file

// EPUBExtractor implements the TextExtractor interface for EPUB files
type EPUBExtractor struct{}

// NewEPUBExtractor creates a new EPUBExtractor instance
func NewEPUBExtractor() *EPUBExtractor {
	return &EPUBExtractor{}
}

// ExtractText extracts text content from an EPUB file
// Implementation is left empty for now as requested
func (e *EPUBExtractor) ExtractText(filepath string) *ExtractionResult {
	// TODO: Implement EPUB text extraction
	return nil
}
