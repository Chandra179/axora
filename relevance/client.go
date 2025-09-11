package relevance

type RelevanceFilterClient interface {
	IsContentRelevant(text string) (bool, float32, error)
}
