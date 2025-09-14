package tor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

type TorClient struct {
	proxyURL      string
	dialer        proxy.Dialer
	controlAddr   string
	controlPass   string
	client        *http.Client
	rotationMutex sync.Mutex
	requestCount  int
	rotationAfter int
}

func NewTorClient(proxyURL string) (*TorClient, error) {
	dialer, err := proxy.SOCKS5("tcp", proxyURL, nil, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}

	// Increased timeouts for Tor network
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
			DisableKeepAlives:     true,
			MaxIdleConns:          0,
			MaxIdleConnsPerHost:   0,
			MaxConnsPerHost:       1,
			IdleConnTimeout:       0,
			ResponseHeaderTimeout: 120 * time.Second, // Increased from 30s
			TLSHandshakeTimeout:   60 * time.Second,  // Added TLS timeout
		},
		Timeout: 300 * time.Second, // Increased total timeout to 5 minutes
	}

	return &TorClient{
		proxyURL:      proxyURL,
		dialer:        dialer,
		client:        httpClient,
		rotationAfter: 10,
		controlAddr:   "axora-tor:9051",
		controlPass:   "test12345",
	}, nil
}

// CreateClientWithTimeouts allows custom timeout configuration
func (tc *TorClient) CreateClientWithTimeouts(dialTimeout, responseTimeout, totalTimeout time.Duration) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				// Add timeout to dial context
				dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
				defer cancel()
				return tc.dialer.(interface {
					DialContext(ctx context.Context, network, addr string) (net.Conn, error)
				}).DialContext(dialCtx, network, addr)
			},
			DisableKeepAlives:     true,
			MaxIdleConns:          0,
			MaxIdleConnsPerHost:   0,
			MaxConnsPerHost:       1,
			IdleConnTimeout:       0,
			ResponseHeaderTimeout: responseTimeout,
			TLSHandshakeTimeout:   60 * time.Second,
		},
		Timeout: totalTimeout,
	}
}

// GetDialContextWithRetry returns a dial context with retry logic
func (tc *TorClient) GetDialContextWithRetry(maxRetries int, retryDelay time.Duration) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		tc.rotationMutex.Lock()
		tc.requestCount++
		shouldRotate := tc.requestCount >= tc.rotationAfter
		if shouldRotate {
			tc.requestCount = 0
		}
		tc.rotationMutex.Unlock()

		if shouldRotate {
			log.Printf("[TOR] Rotating IP after %d requests", tc.rotationAfter)
			if err := tc.rotateIP(); err != nil {
				log.Printf("[TOR] Failed to rotate IP: %v", err)
			} else {
				log.Printf("[TOR] IP rotation successful")
			}
		}

		// Retry logic for connection
		var lastErr error
		for attempt := 0; attempt < maxRetries; attempt++ {
			if attempt > 0 {
				log.Printf("[TOR] Connection attempt %d/%d for %s", attempt+1, maxRetries, addr)
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(retryDelay):
				}
			}

			conn, err := tc.dialer.Dial(network, addr)
			if err != nil {
				lastErr = err
				log.Printf("[TOR] Connection attempt %d failed: %v", attempt+1, err)
				continue
			}
			return conn, nil
		}
		return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
	}
}

// Rest of the methods remain the same...
func (tc *TorClient) signalNewnym() error {
	conn, err := net.DialTimeout("tcp", tc.controlAddr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("controlport dial failed: %w", err)
	}
	defer conn.Close()

	// Set read/write timeout for control connection
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	rd := bufio.NewReader(conn)

	// Authenticate
	if tc.controlPass != "" {
		fmt.Fprintf(conn, "AUTHENTICATE \"%s\"\r\n", tc.controlPass)
	} else {
		fmt.Fprintf(conn, "AUTHENTICATE\r\n")
	}
	line, _ := rd.ReadString('\n')
	if !strings.HasPrefix(line, "250") {
		return fmt.Errorf("control auth failed: %s", line)
	}

	// Request newnym
	fmt.Fprintf(conn, "SIGNAL NEWNYM\r\n")
	line, _ = rd.ReadString('\n')
	if !strings.HasPrefix(line, "250") {
		return fmt.Errorf("SIGNAL NEWNYM failed: %s", line)
	}
	return nil
}

func (tc *TorClient) GetDialContext() func(ctx context.Context, network, addr string) (net.Conn, error) {
	return tc.GetDialContextWithRetry(3, 2*time.Second)
}

func (tc *TorClient) GetHTTPClient() *http.Client {
	return tc.client
}

func (tc *TorClient) SetRotationInterval(requests int) {
	tc.rotationMutex.Lock()
	defer tc.rotationMutex.Unlock()
	tc.rotationAfter = requests
}

func (tc *TorClient) rotateIP() error {
	tc.rotationMutex.Lock()
	defer tc.rotationMutex.Unlock()

	if err := tc.signalNewnym(); err != nil {
		return fmt.Errorf("signal newnym failed: %w", err)
	}

	// Longer sleep for Tor to establish new circuits
	time.Sleep(5 * time.Second)

	// Recreate dialer/client
	dialer, err := proxy.SOCKS5("tcp", tc.proxyURL, nil, proxy.Direct)
	if err != nil {
		return fmt.Errorf("failed to recreate dialer: %w", err)
	}
	tc.dialer = dialer
	tc.client = tc.CreateClientWithTimeouts(30*time.Second, 120*time.Second, 300*time.Second)

	if err := tc.VerifyTorRouting(); err != nil {
		log.Printf("[TOR] Post-rotation verification failed (continuing anyway): %v", err)
		// Don't return error here as verification might fail due to network issues
		// but the rotation itself might have succeeded
	}
	return nil
}

func getDirectIP() (string, error) {
	client := &http.Client{Timeout: 15 * time.Second} // Increased timeout
	resp, err := client.Get("https://httpbin.org/ip")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func (tc *TorClient) VerifyTorRouting() error {
	torIP, err := tc.GetCurrentIP()
	if err != nil {
		return fmt.Errorf("failed to get ip via tor: %w", err)
	}
	directIP, err := getDirectIP()
	if err != nil {
		return fmt.Errorf("failed to verify direct IP (cannot confirm tor routing): %w", err)
	}
	if strings.TrimSpace(torIP) == strings.TrimSpace(directIP) {
		return fmt.Errorf("tor routing failed: tor-ip == direct-ip (%s)", torIP)
	}
	log.Printf("[TOR] routing verified; tor ip != direct ip")
	return nil
}

func (tc *TorClient) TestConnection() error {
	log.Println("Testing Tor connection...")

	// Use a more robust test with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "https://httpbin.org/ip", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := tc.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request through Tor: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("Current IP through Tor: %s", string(body))
	return nil
}

func (tc *TorClient) GetCurrentIP() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "https://httpbin.org/ip", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := tc.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get current IP: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read IP response: %w", err)
	}

	return string(body), nil
}
