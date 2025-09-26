// crawler/config.go
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
	AllowedHosts    []string // New field for host filtering
	IPCheckServices []string
}

// DefaultConfig returns a default crawler configuration
func DefaultConfig() *CrawlerConfig {
	return &CrawlerConfig{
		MaxDepth:        3,
		RequestTimeout:  10800 * time.Second,
		Parallelism:     100,
		IPRotationDelay: 40 * time.Second,
		RequestDelay:    3 * time.Second,
		MaxRetries:      3,
		UserAgent:       "Axora-Crawler/1.0",
		MaxURLVisits:    1,
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
		AllowedHosts: []string{
			"libgen.li",
			"*.booksdl.lc", // Using wildcard pattern for cdn subdomains
		},
		IPCheckServices: []string{
			"https://httpbin.org/ip",
			"https://api.ipify.org?format=text",
			"https://icanhazip.com",
		},
	}
}
