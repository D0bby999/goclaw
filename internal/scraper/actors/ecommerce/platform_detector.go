package ecommerce

import (
	"net/url"
	"strings"
)

// DetectPlatform identifies the ecommerce platform from a URL hostname.
func DetectPlatform(rawURL string) Platform {
	u, err := url.Parse(rawURL)
	if err != nil {
		return PlatformGeneric
	}
	host := strings.ToLower(u.Hostname())

	switch {
	case strings.Contains(host, "amazon."):
		return PlatformAmazon
	case strings.Contains(host, "ebay."):
		return PlatformEbay
	case strings.Contains(host, "walmart."):
		return PlatformWalmart
	default:
		return PlatformGeneric
	}
}

// IsProductURL returns true if the URL pattern matches a known product page.
func IsProductURL(rawURL string, platform Platform) bool {
	switch platform {
	case PlatformAmazon:
		return strings.Contains(rawURL, "/dp/")
	case PlatformEbay:
		return strings.Contains(rawURL, "/itm/")
	case PlatformWalmart:
		return strings.Contains(rawURL, "/ip/")
	default:
		return true
	}
}
