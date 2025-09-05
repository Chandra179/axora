package relevance

type RelevanceFilterClient interface {
	IsURLRelevant(text string) (bool, float32, error)
}
