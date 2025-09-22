package file

// TextExtractor defines the interface for extracting text from files
type TextExtractor interface {
	ExtractText(filepath string) string
}
