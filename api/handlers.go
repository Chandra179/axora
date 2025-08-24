package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ModelServiceClient handles communication with the HuggingFace text-embeddings-inference service
type ModelServiceClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewModelServiceClient creates a new model service client
func NewModelServiceClient(baseURL string) *ModelServiceClient {
	return &ModelServiceClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// EmbeddingRequest represents the request to HuggingFace embeddings service
type EmbeddingRequest struct {
	Inputs    []string `json:"inputs"`
	Normalize bool     `json:"normalize,omitempty"`
}

// EmbeddingHandler handles embedding requests
func (c *ModelServiceClient) EmbeddingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req EmbeddingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	embeddings, err := c.getEmbeddings(req.Inputs)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get embeddings: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(embeddings)
}

// getEmbeddings calls the HuggingFace text-embeddings-inference service
func (c *ModelServiceClient) getEmbeddings(texts []string) ([][]float32, error) {
	req := EmbeddingRequest{
		Inputs:    texts,
		Normalize: true,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/embed", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to make request to model service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("model service returned status %d: %s", resp.StatusCode, string(body))
	}

	var embeddings [][]float32
	if err := json.NewDecoder(resp.Body).Decode(&embeddings); err != nil {
		return nil, fmt.Errorf("failed to decode embeddings response: %w", err)
	}

	return embeddings, nil
}

// SimilarityRequest represents a similarity calculation request
type SimilarityRequest struct {
	Query   string `json:"query"`
	Content string `json:"content"`
}

// SimilarityResponse represents a similarity calculation response
type SimilarityResponse struct {
	Similarity float64 `json:"similarity"`
}

// SimilarityHandler handles similarity calculation requests
func (c *ModelServiceClient) SimilarityHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SimilarityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	similarity, err := c.calculateSimilarity(req.Query, req.Content)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to calculate similarity: %v", err), http.StatusInternalServerError)
		return
	}

	resp := SimilarityResponse{Similarity: similarity}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// calculateSimilarity computes cosine similarity using embeddings
func (c *ModelServiceClient) calculateSimilarity(query, content string) (float64, error) {
	embeddings, err := c.getEmbeddings([]string{query, content})
	if err != nil {
		return 0, err
	}

	if len(embeddings) != 2 {
		return 0, fmt.Errorf("expected 2 embeddings, got %d", len(embeddings))
	}

	return cosineSimilarity(embeddings[0], embeddings[1]), nil
}

// cosineSimilarity calculates cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := range len(a) {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}

	if normA == 0.0 || normB == 0.0 {
		return 0.0
	}

	return dotProduct / (normA * normB)
}