package discovery

import (
	"net/url"
	"sort"
	"strings"
)

// NormalizeURL normalizes a URL for consistent deduplication and comparison.
// - Lowercases scheme and host
// - Sorts query parameters alphabetically
// - Removes fragment (#...)
// - Removes trailing slash (except root "/")
// - Removes default ports (80 for http, 443 for https)
// Returns the original string if parsing fails.
func NormalizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	// Lowercase scheme and host.
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)

	// Remove default ports.
	host := u.Hostname()
	port := u.Port()
	if (u.Scheme == "http" && port == "80") ||
		(u.Scheme == "https" && port == "443") {
		u.Host = host
	}

	// Remove fragment.
	u.Fragment = ""

	// Sort query parameters.
	if u.RawQuery != "" {
		params := u.Query()
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var parts []string
		for _, k := range keys {
			vals := params[k]
			sort.Strings(vals)
			for _, v := range vals {
				parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
			}
		}
		u.RawQuery = strings.Join(parts, "&")
	}

	// Remove trailing slash (except root path).
	if len(u.Path) > 1 && strings.HasSuffix(u.Path, "/") {
		u.Path = strings.TrimSuffix(u.Path, "/")
	}

	return u.String()
}
