package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type POSClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

type posDepRequest struct {
	Text                string `json:"text"`
	Model               string `json:"model"`
	CollapsePunctuation int    `json:"collapse_punctuation,omitempty"`
	CollapsePhrases     int    `json:"collapse_phrases,omitempty"`
}

type posDepResponse struct {
	Arcs  []DepArc `json:"arcs"`
	Words []Token  `json:"words"`
}

type Token struct {
	Tag  string `json:"tag"`
	Text string `json:"text"`
}

type DepArc struct {
	Dir   string `json:"dir"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	Label string `json:"label"`
}

type POSHandler interface {
	Tag(text string) ([]Token, error)
}

func NewPosClient(baseURL string) *POSClient {
	return &POSClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Tag sends text to the /dep endpoint and returns POS tags
func (c *POSClient) Tag(text string) ([]Token, error) {
	reqBody := posDepRequest{
		Text:  text,
		Model: "en",
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/dep", c.BaseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var parsed posDepResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return parsed.Words, nil
}
