package crawler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

// GetPublicIP makes a request to check the current public IP being used
func (w *Crawler) GetPublicIP(ctx context.Context) string {
	for _, service := range w.iPCheckServices {
		ip, err := w.checkService(ctx, service)
		if err != nil {
			w.logger.Error("Failed to check IP",
				zap.String("service", service),
				zap.Error(err))
			continue
		}

		if ip != "" {
			w.logger.Info("IP check successful",
				zap.String("ip", ip),
				zap.String("service", service))
			return ip
		}
	}

	w.logger.Warn("Could not determine public IP")
	return "unknown"
}

// checkService checks IP using a specific service
func (w *Crawler) checkService(ctx context.Context, service string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", service, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Axora-Crawler/1.0")

	resp, err := w.httpClient.Do(req)
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

	return w.parseIPResponse(service, string(body)), nil
}

// parseIPResponse parses the IP from different service response formats
func (w *Crawler) parseIPResponse(service, response string) string {
	ipStr := strings.TrimSpace(response)

	if strings.Contains(service, "httpbin") {
		var data struct {
			Origin string `json:"origin"`
		}
		if err := json.Unmarshal([]byte(response), &data); err == nil {
			return strings.TrimSpace(data.Origin)
		}
	}

	return ipStr
}
