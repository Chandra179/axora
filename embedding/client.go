package embedding

import "context"

type EmbeddingRequest struct {
	Inputs []string `json:"inputs"`
}

type EmbeddingResponse [][]float32

type Client interface {
	// If you send 3 texts, you’ll get 3 vectors.
	// If you send 1 text, you’ll still get 1 vector — but wrapped in a list.
	// Input: ["this is a text"] → list of strings
	// Output: [ [0.12, -0.33, 0.57, ...] ]
	GetEmbeddings(ctx context.Context, texts []string) ([][]float32, error)
}

func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (sqrt(normA) * sqrt(normB))
}

func sqrt(x float32) float32 {
	if x < 0 {
		return 0
	}

	z := float32(1.0)
	for i := 0; i < 10; i++ {
		z -= (z*z - x) / (2 * z)
	}
	return z
}
