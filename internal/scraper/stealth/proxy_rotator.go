package stealth

import (
	"fmt"
	"net/http"
	"net/url"
	"sync/atomic"
)

// ProxyRotator provides round-robin proxy selection from a list of proxies.
type ProxyRotator struct {
	proxies []ProxyConfig
	index   atomic.Uint64
}

// NewProxyRotator creates an empty proxy rotator.
func NewProxyRotator() *ProxyRotator {
	return &ProxyRotator{}
}

// Add parses and validates a proxy URL string, then appends it to the pool.
func (r *ProxyRotator) Add(proxyURL string) error {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("invalid proxy URL %q: %w", proxyURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5" {
		return fmt.Errorf("unsupported proxy scheme %q (want http, https, or socks5)", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("proxy URL %q has no host", proxyURL)
	}

	cfg := ProxyConfig{
		URL:      proxyURL,
		Protocol: u.Scheme,
	}
	if u.User != nil {
		cfg.Username = u.User.Username()
		cfg.Password, _ = u.User.Password()
	}

	r.proxies = append(r.proxies, cfg)
	return nil
}

// GetProxy returns the next proxy URL in round-robin order.
// Returns nil if no proxies have been added.
func (r *ProxyRotator) GetProxy() *url.URL {
	if len(r.proxies) == 0 {
		return nil
	}
	idx := r.index.Add(1) - 1
	cfg := r.proxies[int(idx)%len(r.proxies)]
	u, _ := url.Parse(cfg.URL)
	return u
}

// Count returns the number of configured proxies.
func (r *ProxyRotator) Count() int {
	return len(r.proxies)
}

// GetTransport creates an http.Transport that uses the rotator's proxy function.
func (r *ProxyRotator) GetTransport() *http.Transport {
	return &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return r.GetProxy(), nil
		},
	}
}
