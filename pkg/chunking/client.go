package chunking

type ChunkOutput struct {
	Text   string    `json:"text"`
	Vector []float32 `json:"vector"`
}

type ChunkingClient interface {
	ChunkText(text string) ([]ChunkOutput, error)
}
