package crawler

import (
	"fmt"
	"log"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/standard"
)

// LinkInfo represents a link with metadata for relevance scoring
type LinkInfo struct {
	URL            string
	Title          string
	Description    string
	RelevanceScore float64
}

// RelevanceFilter defines the interface for URL relevance filtering
type RelevanceFilter interface {
	IsURLRelevant(title, metaDescription string) (bool, float64, error)
	GetRelevantLinks(links []LinkInfo) []LinkInfo
	UpdateKeywords(keywords []string) error
	Close() error
}

// BleveRelevanceScorer implements RelevanceFilter using Bleve for TF-IDF scoring
type BleveRelevanceScorer struct {
	keywordIndex bleve.Index
	keywords     []string
	threshold    float64
}

type KeywordDoc struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

// NewBleveRelevanceScorer creates a new Bleve-based relevance scorer
func NewBleveRelevanceScorer(keywords []string, threshold float64) (RelevanceFilter, error) {
	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultAnalyzer = standard.Name

	index, err := bleve.NewMemOnly(indexMapping)
	if err != nil {
		return nil, fmt.Errorf("failed to create index: %w", err)
	}

	scorer := &BleveRelevanceScorer{
		keywordIndex: index,
		keywords:     keywords,
		threshold:    threshold,
	}

	if err := scorer.indexKeywords(); err != nil {
		return nil, fmt.Errorf("failed to index keywords: %w", err)
	}

	return scorer, nil
}

func (brs *BleveRelevanceScorer) indexKeywords() error {
	for i, keyword := range brs.keywords {
		doc := KeywordDoc{
			ID:      fmt.Sprintf("keyword_%d", i),
			Content: keyword,
		}

		if err := brs.keywordIndex.Index(doc.ID, doc); err != nil {
			return fmt.Errorf("failed to index keyword '%s': %w", keyword, err)
		}
	}

	log.Printf("Indexed %d keywords for relevance scoring", len(brs.keywords))
	return nil
}

// IsURLRelevant checks if a URL is relevant based on URL path and meta content
func (brs *BleveRelevanceScorer) IsURLRelevant(title, metaDescription string) (bool, float64, error) {
	content := title + metaDescription
	score, err := brs.calculateRelevanceScore(content)
	if err != nil {
		return false, 0, err
	}

	return score >= brs.threshold, score, nil
}

func (brs *BleveRelevanceScorer) calculateRelevanceScore(content string) (float64, error) {
	if content == "" {
		return 0, nil
	}

	queryString := strings.Join(brs.keywords, " OR ")
	query := bleve.NewQueryStringQuery(queryString)

	searchRequest := bleve.NewSearchRequest(query)
	searchRequest.Size = 10
	searchRequest.Explain = false

	tempDoc := KeywordDoc{
		ID:      "temp_content",
		Content: content,
	}

	if err := brs.keywordIndex.Index(tempDoc.ID, tempDoc); err != nil {
		return 0, fmt.Errorf("failed to index temp document: %w", err)
	}

	searchResult, err := brs.keywordIndex.Search(searchRequest)
	if err != nil {
		return 0, fmt.Errorf("search failed: %w", err)
	}

	brs.keywordIndex.Delete(tempDoc.ID)

	maxScore := 0.0
	for _, hit := range searchResult.Hits {
		if hit.ID == tempDoc.ID {
			maxScore = hit.Score
			break
		}
	}

	return maxScore, nil
}

// UpdateKeywords allows updating the keyword set
func (brs *BleveRelevanceScorer) UpdateKeywords(newKeywords []string) error {
	brs.keywords = newKeywords

	brs.keywordIndex.Close()

	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultAnalyzer = standard.Name

	index, err := bleve.NewMemOnly(indexMapping)
	if err != nil {
		return fmt.Errorf("failed to create new index: %w", err)
	}

	brs.keywordIndex = index

	return brs.indexKeywords()
}

// GetRelevantLinks filters links based on relevance scoring
func (brs *BleveRelevanceScorer) GetRelevantLinks(links []LinkInfo) []LinkInfo {
	relevant := make([]LinkInfo, 0)

	for _, link := range links {
		isRelevant, score, err := brs.IsURLRelevant(link.Title, link.Description)
		if err != nil {
			log.Printf("Error scoring URL %s: %v", link.URL, err)
			continue
		}

		if isRelevant {
			link.RelevanceScore = score
			relevant = append(relevant, link)
			log.Printf("URL %s is relevant (score: %.3f)", link.URL, score)
		}
	}

	return relevant
}

// Close closes the Bleve index
func (brs *BleveRelevanceScorer) Close() error {
	return brs.keywordIndex.Close()
}
