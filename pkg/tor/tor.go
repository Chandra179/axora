package tor

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/proxy"
)

type TorClient struct {
	proxyURL string
	dialer   proxy.Dialer
	client   *http.Client
}

func NewTorClient(proxyURL string) (*TorClient, error) {
	// Parse the SOCKS5 proxy URL
	dialer, err := proxy.SOCKS5("tcp", proxyURL, nil, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}

	// Create HTTP client with Tor proxy
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		},
		Timeout: 30 * time.Second,
	}

	return &TorClient{
		proxyURL: proxyURL,
		dialer:   dialer,
		client:   httpClient,
	}, nil
}

func (tc *TorClient) TestConnection() error {
	log.Println("Testing Tor connection...")

	// Check current IP through Tor
	resp, err := tc.client.Get("https://httpbin.org/ip")
	if err != nil {
		return fmt.Errorf("failed to make request through Tor: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("IP through Tor: %s", string(body))
	return nil
}
