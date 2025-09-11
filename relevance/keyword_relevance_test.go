package relevance

import (
	"testing"
)

func TestKeywordRelevanceFilter_LongContent(t *testing.T) {
	longText := `Apple and banana are fruits that many people enjoy every day. They are found in markets
	around the world and are part of a healthy diet. Eating fruits like apple and banana provides vitamins, 
	fiber, and natural sugars which are beneficial. Many recipes include these fruits, from pies to smoothies,
	and children often love them as snacks neural networks, long short term memory.`

	testCases := []struct {
		name        string
		query       string
		content     string
		expectedRel bool
	}{
		{"LongContentMatch", "neural networks", longText, true},
		{"LongContentMatch", "long short term memory", longText, true},
		{"LongContentNoMatch", "grape,orange", longText, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filter, err := NewKeywordRelevanceFilter(tc.query)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			rel, _, err := filter.IsContentRelevant(tc.content)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if rel != tc.expectedRel {
				t.Errorf("expected relevance %v, got %v", tc.expectedRel, rel)
			}
		})
	}
}
