package crawler

type ChunkingClient interface {
	ChunkText(text string) (string, error)
}
