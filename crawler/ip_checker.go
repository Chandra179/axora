package crawler

import (
	"context"
	"io"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

type IPChecker struct {
	httpClient http.Client
	services   []string
	logger     *zap.Logger
}

// NewIPChecker creates a new IP checker
func NewIPChecker(client http.Client, services []string, logger *zap.Logger) *IPChecker {
	return &IPChecker{
		httpClient: client,
		services:   services,
		logger:     logger,
	}
}

// GetPublicIP makes a request to check the current public IP being used
func (i *IPChecker) GetPublicIP(ctx context.Context) string {
	for _, service := range i.services {
		ip, err := i.checkService(ctx, service)
		if err != nil {
			i.logger.Error("Failed to check IP",
				zap.String("service", service),
				zap.Error(err))
			continue
		}

		if ip != "" {
			i.logger.Info("IP check successful",
				zap.String("ip", ip),
				zap.String("service", service))
			return ip
		}
	}

	i.logger.Warn("Could not determine public IP")
	return "unknown"
}

// checkService checks IP using a specific service
func (i *IPChecker) checkService(ctx context.Context, service string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", service, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Axora-Crawler/1.0")

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return i.parseIPResponse(service, string(body)), nil
}

// parseIPResponse parses the IP from different service response formats
func (i *IPChecker) parseIPResponse(service, response string) string {
	ipStr := strings.TrimSpace(response)

	// For httpbin.org/ip, extract IP from JSON response
	if strings.Contains(service, "httpbin") && strings.Contains(ipStr, "origin") {
		// Parse JSON-like response: {"origin": "1.2.3.4"}
		start := strings.Index(ipStr, "`") + 1
		end := strings.LastIndex(ipStr, "`")
		if start > 0 && end > start {
			ipStr = ipStr[start:end]
			if strings.Contains(ipStr, "origin") {
				parts := strings.Split(ipStr, ": ")
				if len(parts) > 1 {
					ipStr = strings.Trim(parts[1], "`")
				}
			}
		}
	}

	return ipStr
}
