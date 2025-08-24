package crawler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

// RelevanceFilter defines the interface for URL relevance filtering
type RelevanceFilter interface {
	IsURLRelevant(title, metaDescription string) (bool, float64, error)
	Close() error
}


// HTTPModelClient handles communication with the model API service
type HTTPModelClient struct {
	baseURL    string
	httpClient *http.Client
}

// SemanticRelevanceFilter implements relevance filtering using HTTP-based semantic similarity
type SemanticRelevanceFilter struct {
	client         *HTTPModelClient
	queryEmbedding []float32
	threshold      float64
}

// NewHTTPModelClient creates a new HTTP model client
func NewHTTPModelClient(baseURL string) *HTTPModelClient {
	return &HTTPModelClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewSemanticRelevanceFilter creates a new semantic relevance filter using HTTP API
func NewSemanticRelevanceFilter(modelServiceURL, query string, threshold float64) (*SemanticRelevanceFilter, error) {
	// Get model service URL from environment if not provided
	if modelServiceURL == "" {
		modelServiceURL = os.Getenv("MODEL_SERVICE_URL")
		if modelServiceURL == "" {
			modelServiceURL = "http://localhost:8000" // Default fallback
		}
	}

	client := NewHTTPModelClient(modelServiceURL)

	filter := &SemanticRelevanceFilter{
		client:    client,
		threshold: threshold,
	}

	// Pre-compute query embedding
	queryEmbedding, err := client.GetEmbeddings([]string{query})
	if err != nil {
		return nil, fmt.Errorf("failed to compute query embedding: %w", err)
	}
	if len(queryEmbedding) == 0 {
		return nil, fmt.Errorf("no embedding returned for query")
	}
	filter.queryEmbedding = queryEmbedding[0]

	return filter, nil
}

// GetEmbeddings gets embeddings for the given texts from the HuggingFace model service
func (c *HTTPModelClient) GetEmbeddings(texts []string) ([][]float32, error) {
	req := struct {
		Inputs    []string `json:"inputs"`
		Normalize bool     `json:"normalize"`
	}{
		Inputs:    texts,
		Normalize: true,
	}
	
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/embed", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("model service returned status %d", resp.StatusCode)
	}

	var embResp [][]float32
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return embResp, nil
}

// CalculateSimilarity calculates similarity between query and content using embeddings
func (c *HTTPModelClient) CalculateSimilarity(query, content string) (float64, error) {
	embeddings, err := c.GetEmbeddings([]string{query, content})
	if err != nil {
		return 0, fmt.Errorf("failed to get embeddings: %w", err)
	}
	
	if len(embeddings) != 2 {
		return 0, fmt.Errorf("expected 2 embeddings, got %d", len(embeddings))
	}
	
	return cosineSimilarity(embeddings[0], embeddings[1]), nil
}

// IsURLRelevant determines if a URL is relevant based on semantic similarity
func (s *SemanticRelevanceFilter) IsURLRelevant(title, metaDescription string) (bool, float64, error) {
	// Combine title and meta description for content
	content := strings.TrimSpace(title + " " + metaDescription)
	if content == "" {
		return false, 0.0, nil
	}

	// Get content embedding
	contentEmbeddings, err := s.client.GetEmbeddings([]string{content})
	if err != nil {
		return false, 0.0, err
	}
	if len(contentEmbeddings) == 0 {
		return false, 0.0, fmt.Errorf("no embedding returned for content")
	}

	// Calculate cosine similarity
	similarity := cosineSimilarity(s.queryEmbedding, contentEmbeddings[0])

	// Check if above threshold
	isRelevant := similarity >= s.threshold

	return isRelevant, similarity, nil
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

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// Close cleans up the resources (no-op for HTTP client)
func (s *SemanticRelevanceFilter) Close() error {
	return nil
}