package queue

import (
	"net/url"
	"sort"
	"strings"
)

var trackingParams = map[string]bool{
	"utm_source":   true,
	"utm_medium":   true,
	"utm_campaign": true,
	"utm_term":     true,
	"utm_content":  true,
	"fbclid":       true,
	"gclid":        true,
	"ref":          true,
	"source":       true,
}

// NormalizeKey returns a canonical string key for a URL:
// lowercase scheme+host, sorted query params, no fragment, no trailing slash,
// and tracking params removed.
func NormalizeKey(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return strings.ToLower(rawURL)
	}

	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""

	// Strip tracking params and sort the rest.
	q := u.Query()
	for k := range trackingParams {
		q.Del(k)
	}
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sorted := url.Values{}
	for _, k := range keys {
		sorted[k] = q[k]
	}
	u.RawQuery = sorted.Encode()

	// Remove trailing slash unless it's the root path.
	if len(u.Path) > 1 {
		u.Path = strings.TrimRight(u.Path, "/")
	}

	return u.String()
}
