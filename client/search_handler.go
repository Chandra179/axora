package client

import (
	"axora/search"
	"context"
	"fmt"
	"net/http"
)

type SearchClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// ExposeSearchEndpoint exposes /search to public
func SearchHandler(serp *search.SerpApiSearchEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if query == "" {
			http.Error(w, "missing query parameter", http.StatusBadRequest)
			return
		}
		searchResults, _ := serp.Search(context.Background(), &search.SearchRequest{
			Query:    query,
			MaxPages: 2,
		})
		fmt.Println(searchResults)
	}
}
