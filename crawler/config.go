package crawler

import (
	"time"
)

type CrawlerConfig struct {
	MaxDepth        int
	RequestTimeout  time.Duration
	Parallelism     int
	IPRotationDelay time.Duration
	RequestDelay    time.Duration
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
		MaxDepth:        3,
		RequestTimeout:  10800 * time.Second,
		Parallelism:     10,
		IPRotationDelay: 40 * time.Second,
		RequestDelay:    3 * time.Second,
		MaxRetries:      3,
		UserAgent:       "Axora-Crawler/1.0",
		MaxURLVisits:    3,
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
			"ext",
			"curtab",
		},
		AllowedSchemes: []string{"https"},
		IPCheckServices: []string{
			"https://httpbin.org/ip",
			"https://api.ipify.org?format=text",
			"https://icanhazip.com",
		},
	}
}
