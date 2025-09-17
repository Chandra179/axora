package crawler

import (
	"net/url"
	"slices"
)

type URLValidator struct {
	allowedPaths   []string
	allowedParams  map[string]bool
	allowedSchemes []string
}

// NewURLValidator creates a new URL validator with the given configuration
func NewURLValidator(config *CrawlerConfig) *URLValidator {
	return &URLValidator{
		allowedPaths:   config.AllowedPaths,
		allowedParams:  config.GetAllowedParamsMap(),
		allowedSchemes: config.AllowedSchemes,
	}
}

// IsValidDownloadURL validates URL according to the specification
func (v *URLValidator) IsValidDownloadURL(u *url.URL) bool {
	if u.Host == "" {
		return false
	}
	if !slices.Contains(v.allowedSchemes, u.Scheme) {
		return false
	}

	if !slices.Contains(v.allowedPaths, u.Path) {
		return false
	}

	for param := range u.Query() {
		if !v.allowedParams[param] {
			return false
		}
	}

	return true
}
