package httpclient

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/scraper/stealth"
)

const (
	defaultTimeout    = 30 * time.Second
	defaultMaxRetries = 3
)

// Client is a stealth-enabled HTTP client with retry and SSRF protection.
type Client struct {
	httpClient  *http.Client
	fingerprint *stealth.Fingerprint
	fpMgr       *stealth.FingerprintManager
	proxyRot    *stealth.ProxyRotator
	maxRetries  int
}

// Option is a functional option for Client.
type Option func(*Client)

// WithMaxRetries sets the maximum number of retry attempts.
func WithMaxRetries(n int) Option {
	return func(c *Client) { c.maxRetries = n }
}

// WithProxy attaches a proxy rotator to the client transport.
func WithProxy(rot *stealth.ProxyRotator) Option {
	return func(c *Client) { c.proxyRot = rot }
}

// WithFingerprint pins a specific fingerprint for all requests.
func WithFingerprint(fp *stealth.Fingerprint) Option {
	return func(c *Client) { c.fingerprint = fp }
}

// WithFingerprintManager sets a manager that generates fingerprints per request.
func WithFingerprintManager(mgr *stealth.FingerprintManager) Option {
	return func(c *Client) { c.fpMgr = mgr }
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.httpClient.Timeout = d }
}

// NewClient creates an HTTP client with sane defaults and SSRF protection.
func NewClient(opts ...Option) *Client {
	transport := &http.Transport{
		DialContext: ssrfSafeDialer(),
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	c := &Client{
		httpClient: &http.Client{
			Timeout:   defaultTimeout,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("stopped after 10 redirects")
				}
				return nil
			},
		},
		maxRetries: defaultMaxRetries,
	}

	for _, o := range opts {
		o(c)
	}

	// If a proxy rotator is set, wrap the transport with proxy support.
	if c.proxyRot != nil {
		if t, ok := c.httpClient.Transport.(*http.Transport); ok {
			t.Proxy = func(req *http.Request) (*url.URL, error) {
				return c.proxyRot.GetProxy(), nil
			}
		}
	}

	return c
}

// Get performs a stealth GET request with retry logic.
func (c *Client) Get(ctx context.Context, rawURL string) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	return c.Do(ctx, req)
}

// Do applies stealth headers and executes the request with exponential backoff retry.
func (c *Client) Do(ctx context.Context, req *http.Request) (*Response, error) {
	fp := c.resolveFingerprint()
	applyStealthHeaders(req, fp)

	var (
		resp *http.Response
		err  error
	)

	backoff := time.Second
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			if werr := stealth.WaitWithJitter(ctx, backoff); werr != nil {
				return nil, werr
			}
			backoff *= 2
		}

		// Clone the request so headers/state are fresh on each attempt.
		clone := req.Clone(ctx)
		resp, err = c.httpClient.Do(clone)
		if err != nil {
			// Network error — retry.
			continue
		}

		// Retry on 429 (rate-limit) with Retry-After header.
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"), backoff)
			_ = resp.Body.Close()
			resp = nil
			backoff = retryAfter
			continue
		}
		// No retry on other 4xx client errors.
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			break
		}
		// Retry on 5xx server errors.
		if resp.StatusCode >= 500 {
			_ = resp.Body.Close()
			resp = nil
			continue
		}
		// Success (1xx/2xx/3xx).
		break
	}

	if err != nil {
		return nil, fmt.Errorf("request failed after %d attempts: %w", c.maxRetries+1, err)
	}
	if resp == nil {
		return nil, fmt.Errorf("request failed after %d attempts: server error", c.maxRetries+1)
	}
	defer resp.Body.Close()

	// Limit body read to 10MB to prevent OOM on large responses.
	const maxBodySize = 10 << 20
	bodyBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if readErr != nil {
		return nil, fmt.Errorf("read response body: %w", readErr)
	}

	finalURL := resp.Request.URL.String()

	return &Response{
		StatusCode:  resp.StatusCode,
		Body:        string(bodyBytes),
		Headers:     resp.Header,
		ContentType: resp.Header.Get("Content-Type"),
		URL:         finalURL,
	}, nil
}

// parseRetryAfter parses the Retry-After header value (seconds) and returns
// a duration. Falls back to the provided default if header is missing or invalid.
// Caps at 60s to prevent excessive waits.
func parseRetryAfter(header string, fallback time.Duration) time.Duration {
	if header == "" {
		return fallback
	}
	secs, err := strconv.Atoi(header)
	if err != nil || secs <= 0 {
		return fallback
	}
	d := time.Duration(secs) * time.Second
	if d > 60*time.Second {
		d = 60 * time.Second
	}
	return d
}

// resolveFingerprint returns a pinned fingerprint, generates one from the manager,
// or returns an empty fingerprint if neither is configured.
func (c *Client) resolveFingerprint() *stealth.Fingerprint {
	if c.fingerprint != nil {
		return c.fingerprint
	}
	if c.fpMgr != nil {
		fp := c.fpMgr.Generate()
		return &fp
	}
	return &stealth.Fingerprint{}
}

// applyStealthHeaders sets User-Agent and stealth headers on the request.
func applyStealthHeaders(req *http.Request, fp *stealth.Fingerprint) {
	if fp.UserAgent != "" {
		req.Header.Set("User-Agent", fp.UserAgent)
	}
	headers := stealth.BuildStealthHeaders(*fp)
	for k, v := range headers {
		if req.Header.Get(k) == "" {
			req.Header.Set(k, v)
		}
	}
}

// ssrfSafeDialer returns a DialContext func that blocks private/loopback IPs.
func ssrfSafeDialer() func(ctx context.Context, network, addr string) (net.Conn, error) {
	d := &net.Dialer{Timeout: 10 * time.Second}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}

		ips, err := net.DefaultResolver.LookupHost(ctx, host)
		if err != nil {
			return nil, err
		}

		for _, ipStr := range ips {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				continue
			}
			if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				return nil, fmt.Errorf("SSRF protection: blocked request to private/loopback address %s", ipStr)
			}
		}

		return d.DialContext(ctx, network, net.JoinHostPort(ips[0], port))
	}
}
