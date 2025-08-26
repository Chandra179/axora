package crawler

import (
	"sync"
)

type LoopDetector struct {
	visited   map[string]int
	maxVisits int
	mutex     sync.RWMutex
}

func NewLoopDetector(maxVisits int) *LoopDetector {
	return &LoopDetector{
		visited:   make(map[string]int),
		maxVisits: maxVisits,
	}
}

func (ld *LoopDetector) CheckLoop(url string) bool {
	ld.mutex.Lock()
	defer ld.mutex.Unlock()
	currentVisits := ld.visited[url]
	return currentVisits >= ld.maxVisits
}

func (ld *LoopDetector) IncVisit(url string) {
	ld.mutex.Lock()
	defer ld.mutex.Unlock()
	ld.visited[url]++
}
