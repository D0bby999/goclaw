package stealth

// Fingerprint represents a browser fingerprint with UA, headers, and screen info.
type Fingerprint struct {
	ID        string
	UserAgent string
	Headers   map[string]string
	Screen    ScreenSize
}

// ScreenSize holds browser viewport dimensions.
type ScreenSize struct {
	Width  int
	Height int
}

// ProxyConfig holds proxy connection details.
type ProxyConfig struct {
	URL      string
	Protocol string // http, socks5
	Username string
	Password string
}
