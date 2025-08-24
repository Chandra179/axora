package api

import (
	"log"
	"net/http"
	"os"
)

// Server represents the API server
type Server struct {
	modelClient *ModelServiceClient
	port        string
}

// NewServer creates a new API server
func NewServer(modelServiceURL, port string) *Server {
	return &Server{
		modelClient: NewModelServiceClient(modelServiceURL),
		port:        port,
	}
}

// Start starts the API server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Register API endpoints
	mux.HandleFunc("/api/embed", s.modelClient.EmbeddingHandler)
	mux.HandleFunc("/api/similarity", s.modelClient.SimilarityHandler)

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Printf("Starting API server on port %s", s.port)
	return http.ListenAndServe(":"+s.port, mux)
}

// GetModelServiceURL returns the model service URL from environment or default
func GetModelServiceURL() string {
	url := os.Getenv("MODEL_SERVICE_URL")
	if url == "" {
		url = "http://localhost:8000" // Default fallback
	}
	return url
}

// GetAPIPort returns the API port from environment or default
func GetAPIPort() string {
	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080" // Default port
	}
	return port
}
