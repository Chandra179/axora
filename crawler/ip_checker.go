package crawler

import (
	"context"
	"io"
	"net/http"
	"strings"
)

func GetPublicIP(ctx context.Context, httpClient *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.ipify.org?format=text", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Axora-Crawler/1.0")

	resp, err := httpClient.Do(req)
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

	ipStr := strings.TrimSpace(string(body))

	return ipStr, nil
}
