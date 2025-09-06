package chunking

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type SemanticChunking struct {
	BaseURL    string
	HTTPClient *http.Client
}

type ChunkRequest struct {
	Text string `json:"text"`
}

type ChunkResponse struct {
	Text   string    `json:"text"`
	Vector []float64 `json:"vector"`
}

func NewSemanticChunking(baseURL string) *SemanticChunking {
	return &SemanticChunking{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *SemanticChunking) ChunkText(text string) ([]ChunkOutput, error) {
	requestBody := ChunkRequest{
		Text: text,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.BaseURL + "/chunk"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var chunks []ChunkOutput
	if err := json.NewDecoder(resp.Body).Decode(&chunks); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return chunks, nil
}
