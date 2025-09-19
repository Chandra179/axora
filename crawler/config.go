package crawler

import (
	"time"
)

type CrawlerConfig struct {
	MaxDepth        int
	RequestTimeout  time.Duration
	Parallelism     int
	Delay           time.Duration
	MaxRetries      int
	UserAgent       string
	MaxURLVisits    int
	AllowedPaths    []string
	AllowedParams   []string
	AllowedSchemes  []string
	IPCheckServices []string
}

// DefaultConfig returns a default crawler configuration
func DefaultConfig() *CrawlerConfig {
	return &CrawlerConfig{
		MaxDepth:       5,
		RequestTimeout: 300 * time.Second,
		Parallelism:    2,
		Delay:          600 * time.Second,
		MaxRetries:     3,
		UserAgent:      "Axora-Crawler/1.0",
		MaxURLVisits:   1,
		AllowedPaths: []string{
			"/index.php",
			"/edition.php",
			"/ads.php",
			"/get.php",
		},
		AllowedParams: []string{
			"req",
			"id",
			"md5",
			"downloadname",
			"key",
		},
		AllowedSchemes: []string{"https"},
		IPCheckServices: []string{
			"https://httpbin.org/ip",
			"https://api.ipify.org?format=text",
			"https://icanhazip.com",
		},
	}
}

// GetAllowedParamsMap returns allowed parameters as a map for faster lookup
func (c *CrawlerConfig) GetAllowedParamsMap() map[string]bool {
	paramsMap := make(map[string]bool, len(c.AllowedParams))
	for _, param := range c.AllowedParams {
		paramsMap[param] = true
	}
	return paramsMap
}
