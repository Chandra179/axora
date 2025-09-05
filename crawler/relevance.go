package crawler

type RelevanceFilter interface {
	IsURLRelevant(text string) (bool, float32, error)
}
