package crawler

type RelevanceFilter interface {
	IsURLRelevant(content string) (bool, float32, error)
}
