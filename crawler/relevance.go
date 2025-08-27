package crawler

type RelevanceFilter interface {
	IsURLRelevant(content string) (bool, float64, error)
}
