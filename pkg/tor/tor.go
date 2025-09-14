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
	controlAddr   string // e.g. "127.0.0.1:9051"
	controlPass   string // empty if no control auth
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

	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
			DisableKeepAlives:     true, // Important for IP rotation
			MaxIdleConns:          0,
			MaxIdleConnsPerHost:   0,
			MaxConnsPerHost:       1,
			IdleConnTimeout:       0,
			ResponseHeaderTimeout: 30 * time.Second,
		},
		Timeout: 60 * time.Second,
	}

	return &TorClient{
		proxyURL:      proxyURL,
		dialer:        dialer,
		client:        httpClient,
		rotationAfter: 10, // Rotate IP every 10 requests by default
		controlAddr:   "axora-tor:9051",
		controlPass:   "test12345",
	}, nil
}

// signalNewnym issues AUTH (if needed) and SIGNAL NEWNYM to control port, returns error if not 250 OK
func (tc *TorClient) signalNewnym() error {
	conn, err := net.Dial("tcp", tc.controlAddr)
	if err != nil {
		return fmt.Errorf("controlport dial failed: %w", err)
	}
	defer conn.Close()

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

		return tc.dialer.Dial(network, addr)
	}
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

	// small sleep for Tor to establish new circuits
	time.Sleep(2 * time.Second)

	// now recreate dialer/client (optional, but fine)
	dialer, err := proxy.SOCKS5("tcp", tc.proxyURL, nil, proxy.Direct)
	if err != nil {
		return fmt.Errorf("failed to recreate dialer: %w", err)
	}
	tc.dialer = dialer
	tc.client = &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return tc.dialer.Dial(network, addr)
			},
			DisableKeepAlives:     true,
			ResponseHeaderTimeout: 30 * time.Second,
		},
		Timeout: 60 * time.Second,
	}

	if err := tc.VerifyTorRouting(); err != nil {
		return fmt.Errorf("post-rotation verification failed: %w", err)
	}
	return nil
}

func getDirectIP() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
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

// VerifyTorRouting ensures the IP seen via Tor is different from direct IP (fail-closed)
func (tc *TorClient) VerifyTorRouting() error {
	torIP, err := tc.GetCurrentIP()
	if err != nil {
		return fmt.Errorf("failed to get ip via tor: %w", err)
	}
	directIP, err := getDirectIP()
	if err != nil {
		// if we cannot determine direct IP, be conservative: warn and return error
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

	log.Printf("Current IP through Tor: %s", string(body))
	return nil
}

func (tc *TorClient) GetCurrentIP() (string, error) {
	resp, err := tc.client.Get("https://httpbin.org/ip")
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
