package tor

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

type TorClient struct {
	proxyURL     string
	controlAddr  string
	dialer       proxy.Dialer
	client       *http.Client
	mutex        sync.RWMutex
	requestCount int
	rotateAfter  int // Rotate IP after N requests
	lastRotation time.Time
}

func NewTorClient(proxyURL, controlAddr string, rotateAfter int) (*TorClient, error) {
	tc := &TorClient{
		proxyURL:     proxyURL,
		controlAddr:  controlAddr,
		rotateAfter:  rotateAfter,
		lastRotation: time.Now(),
	}

	if err := tc.createNewConnection(); err != nil {
		return nil, err
	}

	return tc, nil
}

func (tc *TorClient) createNewConnection() error {
	// Parse the SOCKS5 proxy URL
	dialer, err := proxy.SOCKS5("tcp", tc.proxyURL, nil, proxy.Direct)
	if err != nil {
		return fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}

	// Create HTTP client with Tor proxy
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
			// Add these for better connection management
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
		Timeout: 30 * time.Second,
	}

	tc.mutex.Lock()
	tc.dialer = dialer
	tc.client = httpClient
	tc.mutex.Unlock()

	return nil
}

// GetNewTorIP requests a new Tor circuit (new IP)
func (tc *TorClient) GetNewTorIP() error {
	conn, err := net.Dial("tcp", tc.controlAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to Tor control port: %w", err)
	}
	defer conn.Close()

	// Send NEWNYM signal to get new circuit
	_, err = conn.Write([]byte("AUTHENTICATE \"\"\r\n"))
	if err != nil {
		return fmt.Errorf("failed to authenticate: %w", err)
	}

	// Read authentication response
	buffer := make([]byte, 1024)
	_, err = conn.Read(buffer)
	if err != nil {
		return fmt.Errorf("failed to read auth response: %w", err)
	}

	// Send NEWNYM command
	_, err = conn.Write([]byte("SIGNAL NEWNYM\r\n"))
	if err != nil {
		return fmt.Errorf("failed to send NEWNYM: %w", err)
	}

	// Read NEWNYM response
	_, err = conn.Read(buffer)
	if err != nil {
		return fmt.Errorf("failed to read NEWNYM response: %w", err)
	}

	// Send QUIT
	conn.Write([]byte("QUIT\r\n"))

	tc.mutex.Lock()
	tc.lastRotation = time.Now()
	tc.requestCount = 0
	tc.mutex.Unlock()

	log.Printf("[TOR] IP rotated successfully")
	return nil
}

// RotateIPIfNeeded rotates IP if conditions are met
func (tc *TorClient) RotateIPIfNeeded() error {
	tc.mutex.Lock()
	shouldRotate := tc.requestCount >= tc.rotateAfter
	tc.mutex.Unlock()

	if shouldRotate {
		if err := tc.GetNewTorIP(); err != nil {
			return fmt.Errorf("failed to rotate IP: %w", err)
		}

		// Wait a bit for the new circuit to be established
		time.Sleep(2 * time.Second)

		// Recreate the connection with new circuit
		if err := tc.createNewConnection(); err != nil {
			return fmt.Errorf("failed to create new connection: %w", err)
		}
	}

	return nil
}

// Do makes an HTTP request and handles IP rotation
func (tc *TorClient) Do(req *http.Request) (*http.Response, error) {
	// Check if we need to rotate IP before request
	if err := tc.RotateIPIfNeeded(); err != nil {
		log.Printf("[TOR] Warning: IP rotation failed: %v", err)
	}

	tc.mutex.RLock()
	client := tc.client
	tc.mutex.RUnlock()

	resp, err := client.Do(req)

	tc.mutex.Lock()
	tc.requestCount++
	tc.mutex.Unlock()

	return resp, err
}

// Get is a convenience method for GET requests
func (tc *TorClient) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return tc.Do(req)
}

// GetCurrentIP returns the current external IP (for testing)
func (tc *TorClient) GetCurrentIP() (string, error) {
	resp, err := tc.Get("https://httpbin.org/ip")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
