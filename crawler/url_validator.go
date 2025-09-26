// crawler/url_validator.go
package crawler

import (
	"net/url"
	"regexp"
	"slices"
	"strings"
)

type URLValidator struct {
	allowedPaths   []string
	allowedParams  []string
	allowedSchemes []string
	allowedHosts   []string
}

func NewURLValidator(config *CrawlerConfig) *URLValidator {
	return &URLValidator{
		allowedPaths:   config.AllowedPaths,
		allowedParams:  config.AllowedParams,
		allowedSchemes: config.AllowedSchemes,
		allowedHosts:   config.AllowedHosts,
	}
}

func (v *URLValidator) IsValidDownloadURL(u *url.URL) bool {
	if u.Host == "" {
		return false
	}

	if v.isHostAllowed(u.Host) {
		return true
	}

	if slices.Contains(v.allowedSchemes, u.Scheme) {
		return true
	}

	if slices.Contains(v.allowedPaths, u.Path) {
		return true
	}

	for param := range u.Query() {
		if slices.Contains(v.allowedParams, param) {
			return true
		}
	}

	return false
}

func (v *URLValidator) isHostAllowed(host string) bool {
	for _, allowedHost := range v.allowedHosts {
		if v.matchesHostPattern(host, allowedHost) {
			return true
		}
	}
	return false
}

func (v *URLValidator) matchesHostPattern(host, pattern string) bool {
	if host == pattern {
		return true
	}

	// Handle wildcard patterns
	if strings.Contains(pattern, "*") {
		regexPattern := strings.ReplaceAll(pattern, "*", ".*")
		regexPattern = "^" + regexPattern + "$"

		matched, err := regexp.MatchString(regexPattern, host)
		if err != nil {
			return false
		}
		return matched
	}

	if strings.HasSuffix(pattern, ".booksdl.lc") && strings.HasSuffix(host, ".booksdl.lc") {
		hostParts := strings.Split(host, ".")
		if len(hostParts) >= 3 {
			subdomain := hostParts[0]
			// Check if subdomain matches cdn pattern (cdn + number)
			cdnPattern := regexp.MustCompile(`^cdn\d*$`)
			return cdnPattern.MatchString(subdomain)
		}
	}

	return false
}
