package search

import "context"

type SearchResult struct {
	URL         string            `json:"url"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type SearchRequest struct {
	Query    string            `json:"query"`
	MaxPages int               `json:"max_pages,omitempty"`
	Options  map[string]string `json:"options,omitempty"`
}

type SearchEngine interface {
	Search(ctx context.Context, req *SearchRequest) ([]SearchResult, error)
}