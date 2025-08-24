package crawler

import (
	"fmt"
	"log"
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// LinkInfo represents a link with metadata for relevance scoring
type LinkInfo struct {
	URL            string
	Title          string
	Description    string
	AnchorText     string
	Headings       string
	RelevanceScore float64
}

// RelevanceFilter defines the interface for URL relevance filtering
type RelevanceFilter interface {
	IsURLRelevant(title, metaDescription, anchorText, headings string) (bool, float64, error)
	Close() error
}

// SemanticRelevanceScorer implements RelevanceFilter using Sentence-BERT embeddings
type SemanticRelevanceScorer struct {
	query         string
	queryEmbedding []float32
	threshold     float64
	// session would hold ONNX runtime session in real implementation
	session       interface{}
	modelPath     string
}

// NewSemanticRelevanceScorer creates a new Sentence-BERT based relevance scorer
func NewSemanticRelevanceScorer(query string, threshold float64, modelPath string) (RelevanceFilter, error) {
	// In real implementation, initialize ONNX runtime session here
	// session, err := onnxruntime.NewSession(modelPath)
	// For now, using mock implementation
	var session interface{} = nil

	scorer := &SemanticRelevanceScorer{
		query:     query,
		threshold: threshold,
		session:   session,
		modelPath: modelPath,
	}

	// Generate query embedding once
	queryEmbedding, err := scorer.generateEmbedding(query)
	if err != nil {
		// In real implementation: session.Destroy()
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}
	scorer.queryEmbedding = queryEmbedding

	return scorer, nil
}

// IsURLRelevant checks if a URL is relevant using semantic similarity
func (srs *SemanticRelevanceScorer) IsURLRelevant(title, metaDescription, anchorText, headings string) (bool, float64, error) {
	// Extract content with weighted priorities as per README
	content := srs.combineContent(title, metaDescription, anchorText, headings)
	
	// Generate embedding for content
	contentEmbedding, err := srs.generateEmbedding(content)
	if err != nil {
		return false, 0, fmt.Errorf("failed to generate content embedding: %w", err)
	}

	// Calculate cosine similarity
	score := srs.cosineSimilarity(srs.queryEmbedding, contentEmbedding)
	isRelevant := score >= srs.threshold

	// Log detailed processing information
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	log.Printf("[%s] Semantic - Title: '%s', Meta: '%s', Anchor: '%s', Query: '%s', Score: %.3f, Threshold: %.3f, Relevant: %v",
		timestamp, title, metaDescription, anchorText, srs.query, score, srs.threshold, isRelevant)

	return isRelevant, score, nil
}

// combineContent combines HTML elements with weighted priorities
func (srs *SemanticRelevanceScorer) combineContent(title, metaDescription, anchorText, headings string) string {
	var parts []string
	
	// Primary sources with repetition for weighting
	if title != "" {
		// Weight: 35% - repeat 3-4 times
		for i := 0; i < 3; i++ {
			parts = append(parts, title)
		}
	}
	
	if metaDescription != "" {
		// Weight: 30% - repeat 3 times
		for i := 0; i < 3; i++ {
			parts = append(parts, metaDescription)
		}
	}
	
	if anchorText != "" {
		// Weight: 25% - repeat 2-3 times
		for i := 0; i < 2; i++ {
			parts = append(parts, anchorText)
		}
	}
	
	// Secondary sources
	if headings != "" {
		// Weight: 10% - include once
		parts = append(parts, headings)
	}
	
	return strings.Join(parts, " ")
}

// generateEmbedding generates a 384-dimensional embedding using all-MiniLM-L6-v2
func (srs *SemanticRelevanceScorer) generateEmbedding(text string) ([]float32, error) {
	if text == "" {
		return make([]float32, 384), nil
	}
	
	// Preprocess text: clean and tokenize
	processedText := srs.preprocessText(text)
	
	// For now, return a mock embedding - this would be replaced with actual ONNX inference
	// In a real implementation, you would:
	// 1. Tokenize the text using the model's tokenizer
	// 2. Create input tensors
	// 3. Run inference through the ONNX model
	// 4. Extract the embedding from the output
	
	// Mock implementation - generates deterministic embedding based on text content
	return srs.generateMockEmbedding(processedText), nil
}

// preprocessText cleans and prepares text for embedding generation
func (srs *SemanticRelevanceScorer) preprocessText(text string) string {
	// Remove HTML tags if any
	re := regexp.MustCompile(`<[^>]*>`)
	clean := re.ReplaceAllString(text, " ")
	
	// Normalize whitespace
	re = regexp.MustCompile(`\s+`)
	clean = re.ReplaceAllString(clean, " ")
	
	// Trim and convert to lowercase
	clean = strings.ToLower(strings.TrimSpace(clean))
	
	// Limit length to reasonable size for processing
	if len(clean) > 512 {
		clean = clean[:512]
	}
	
	return clean
}

// generateMockEmbedding creates a deterministic mock embedding for testing
func (srs *SemanticRelevanceScorer) generateMockEmbedding(text string) []float32 {
	embedding := make([]float32, 384)
	
	// Simple hash-based mock embedding generation
	textBytes := []byte(text)
	for i := range embedding {
		hash := 0
		for j, b := range textBytes {
			hash += int(b) * (i + j + 1)
		}
		embedding[i] = float32(hash%1000-500) / 1000.0
	}
	
	// Normalize the embedding vector
	return srs.normalizeVector(embedding)
}

// normalizeVector normalizes a vector to unit length
func (srs *SemanticRelevanceScorer) normalizeVector(vector []float32) []float32 {
	var magnitude float32
	for _, v := range vector {
		magnitude += v * v
	}
	magnitude = float32(math.Sqrt(float64(magnitude)))
	
	if magnitude == 0 {
		return vector
	}
	
	normalized := make([]float32, len(vector))
	for i, v := range vector {
		normalized[i] = v / magnitude
	}
	return normalized
}

// cosineSimilarity calculates cosine similarity between two embedding vectors
func (srs *SemanticRelevanceScorer) cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	
	var dotProduct, normA, normB float32
	
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	
	normA = float32(math.Sqrt(float64(normA)))
	normB = float32(math.Sqrt(float64(normB)))
	
	if normA == 0 || normB == 0 {
		return 0
	}
	
	similarity := float64(dotProduct / (normA * normB))
	
	// Ensure similarity is in range [-1, 1]
	similarity = math.Max(-1.0, math.Min(1.0, similarity))
	
	// Convert to range [0, 1] for easier threshold handling
	return (similarity + 1.0) / 2.0
}

// extractURLTokens extracts meaningful words from URL structure
func (srs *SemanticRelevanceScorer) extractURLTokens(urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	
	// Extract path components
	pathParts := strings.Split(parsedURL.Path, "/")
	var tokens []string
	
	for _, part := range pathParts {
		if part == "" {
			continue
		}
		
		// Split on common delimiters
		subParts := regexp.MustCompile(`[-_]+`).Split(part, -1)
		for _, subPart := range subParts {
			if len(subPart) > 2 && !isNumeric(subPart) {
				tokens = append(tokens, subPart)
			}
		}
	}
	
	return strings.Join(tokens, " ")
}

// isNumeric checks if a string contains only numbers
func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Close implements the RelevanceFilter interface
func (srs *SemanticRelevanceScorer) Close() error {
	// In real implementation: srs.session.Destroy()
	// Mock implementation needs no cleanup
	return nil
}
