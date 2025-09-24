package file

type ExtractionResult struct {
	Text     string `json:"text"`
	FilePath string `json:"filepath"`
	Pages    int    `json:"pages,omitempty"`    // For PDFs
	Chapters int    `json:"chapters,omitempty"` // For EPUBs
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
}

type TextExtractor interface {
	ExtractText(filepath string) *ExtractionResult
}
