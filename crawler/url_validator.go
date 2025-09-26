package crawler

import (
	"net/url"
	"slices"
)

type URLValidator struct {
	allowedPaths   []string
	allowedParams  []string
	allowedSchemes []string
}

// NewURLValidator creates a new URL validator with the given configuration
func NewURLValidator(config *CrawlerConfig) *URLValidator {
	return &URLValidator{
		allowedPaths:   config.AllowedPaths,
		allowedParams:  config.AllowedParams,
		allowedSchemes: config.AllowedSchemes,
	}
}

// IsValidDownloadURL validates URL according to the specification
func (v *URLValidator) IsValidDownloadURL(u *url.URL) bool {
	res := u.Host != ""

	if slices.Contains(v.allowedSchemes, u.Scheme) {
		res = res && true
	}

	if slices.Contains(v.allowedPaths, u.Path) {
		return res && true
	}

	for param := range u.Query() {
		if slices.Contains(v.allowedParams, param) {
			return res && true
		}
	}

	return false
}
