package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

type SerpApiSearchEngine struct {
	client *http.Client
	apiKey string
}

type serpApiResponse struct {
	OrganicResults []struct {
		Position int    `json:"position"`
		Title    string `json:"title"`
		Link     string `json:"link"`
		Snippet  string `json:"snippet"`
	} `json:"organic_results"`
	SearchMetadata struct {
		Status string `json:"status"`
	} `json:"search_metadata"`
}

func NewSerpApiSearchEngine(apiKey string) *SerpApiSearchEngine {
	return &SerpApiSearchEngine{
		client: &http.Client{},
		apiKey: apiKey,
	}
}

func (s *SerpApiSearchEngine) Search(ctx context.Context, req *SearchRequest) ([]SearchResult, error) {
	var allResults []SearchResult

	maxPages := req.MaxPages
	if maxPages == 0 {
		maxPages = 10
	}

	for i := range maxPages {
		start := i * 10

		params := url.Values{}
		params.Set("engine", "google")
		params.Set("q", req.Query)
		params.Set("api_key", s.apiKey)
		params.Set("start", strconv.Itoa(start))
		params.Set("num", "10")

		apiURL := "https://serpapi.com/search?" + params.Encode()

		httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := s.client.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("failed to make request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
		}

		var searchResp serpApiResponse
		if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		for _, item := range searchResp.OrganicResults {
			result := SearchResult{
				URL:         item.Link,
				Title:       item.Title,
				Description: item.Snippet,
				Metadata: map[string]string{
					"page":     strconv.Itoa(i + 1),
					"position": strconv.Itoa(item.Position),
					"query":    req.Query,
				},
			}
			allResults = append(allResults, result)
		}

		if len(searchResp.OrganicResults) == 0 {
			break
		}
	}

	return allResults, nil
}
