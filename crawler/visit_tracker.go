package crawler

import (
	"sync"
)

type VisitTracker struct {
	visitedURL   map[string]int
	maxURLVisits int
	mutex        sync.RWMutex
}

// NewVisitTracker creates a new visit tracker
func NewVisitTracker(maxVisits int) *VisitTracker {
	return &VisitTracker{
		visitedURL:   make(map[string]int),
		maxURLVisits: maxVisits,
	}
}

// ShouldVisit checks if a URL should be visited based on visit count
func (vt *VisitTracker) ShouldVisit(url string) bool {
	vt.mutex.RLock()
	defer vt.mutex.RUnlock()

	currentVisits := vt.visitedURL[url]
	return currentVisits < vt.maxURLVisits
}

// RecordVisit records a visit to a URL
func (vt *VisitTracker) RecordVisit(url string) {
	vt.mutex.Lock()
	defer vt.mutex.Unlock()

	vt.visitedURL[url]++
}

// GetTotalVisits returns the total number of visits recorded
func (vt *VisitTracker) GetTotalVisits() int {
	vt.mutex.RLock()
	defer vt.mutex.RUnlock()

	total := 0
	for _, count := range vt.visitedURL {
		total += count
	}
	return total
}

// GetUniqueURLsCount returns the number of unique URLs visited
func (vt *VisitTracker) GetUniqueURLsCount() int {
	vt.mutex.RLock()
	defer vt.mutex.RUnlock()

	return len(vt.visitedURL)
}
