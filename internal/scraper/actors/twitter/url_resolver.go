package twitter

import (
	"regexp"
	"strings"
)

var (
	// tweetURLPattern matches x.com/twitter.com/<user>/status/<id>
	tweetURLPattern = regexp.MustCompile(`(?i)(?:twitter\.com|x\.com)/([^/]+)/status/(\d+)`)
	// profileURLPattern matches x.com/twitter.com/<user> (no /status/)
	profileURLPattern = regexp.MustCompile(`(?i)(?:twitter\.com|x\.com)/([^/?#]+)(?:[/?#]|$)`)
)

// ResolveURL determines whether a URL points to a tweet or profile.
// Returns (urlType, id) where urlType is "tweet" or "profile".
func ResolveURL(rawURL string) (urlType string, id string) {
	if m := tweetURLPattern.FindStringSubmatch(rawURL); len(m) == 3 {
		return "tweet", m[2]
	}
	if m := profileURLPattern.FindStringSubmatch(rawURL); len(m) == 2 {
		handle := m[1]
		// Skip reserved Twitter paths.
		reserved := map[string]bool{
			"home": true, "explore": true, "notifications": true,
			"messages": true, "settings": true, "search": true, "i": true,
		}
		if !reserved[strings.ToLower(handle)] {
			return "profile", handle
		}
	}
	return "", ""
}

// NormalizeHandle strips the leading @ and extracts a handle from a URL or plain string.
func NormalizeHandle(input string) string {
	input = strings.TrimSpace(input)

	// If it looks like a URL, extract the handle segment.
	if strings.Contains(input, "twitter.com") || strings.Contains(input, "x.com") {
		if m := profileURLPattern.FindStringSubmatch(input); len(m) == 2 {
			return strings.TrimPrefix(m[1], "@")
		}
	}

	// Plain handle: strip leading @.
	return strings.TrimPrefix(input, "@")
}
