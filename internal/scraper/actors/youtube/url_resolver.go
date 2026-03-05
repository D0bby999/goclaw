package youtube

import (
	"net/url"
	"strings"
)

// ResolveURL parses a YouTube URL and returns its type and ID.
// Types: "video", "channel", "playlist", "short", "unknown".
func ResolveURL(rawURL string) (urlType, id string) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "unknown", ""
	}

	host := strings.ToLower(u.Host)
	// Handle youtu.be short links
	if strings.Contains(host, "youtu.be") {
		id := strings.TrimPrefix(u.Path, "/")
		if id != "" {
			return "video", id
		}
		return "unknown", ""
	}

	if !strings.Contains(host, "youtube.com") {
		return "unknown", ""
	}

	path := u.Path
	q := u.Query()

	switch {
	case strings.HasPrefix(path, "/shorts/"):
		return "short", strings.TrimPrefix(path, "/shorts/")

	case path == "/watch":
		if v := q.Get("v"); v != "" {
			return "video", v
		}
		return "unknown", ""

	case strings.HasPrefix(path, "/channel/"):
		return "channel", strings.TrimPrefix(path, "/channel/")

	case strings.HasPrefix(path, "/@"):
		return "channel", strings.TrimPrefix(path, "/@")

	case strings.HasPrefix(path, "/c/"):
		return "channel", strings.TrimPrefix(path, "/c/")

	case strings.HasPrefix(path, "/user/"):
		return "channel", strings.TrimPrefix(path, "/user/")

	case path == "/playlist":
		if list := q.Get("list"); list != "" {
			return "playlist", list
		}
		return "unknown", ""

	default:
		// Try treating last path segment as handle
		seg := strings.Trim(path, "/")
		if seg != "" && !strings.Contains(seg, "/") {
			return "channel", seg
		}
		return "unknown", ""
	}
}

// ExtractVideoID pulls the video ID from a YouTube URL.
func ExtractVideoID(rawURL string) string {
	t, id := ResolveURL(rawURL)
	if t == "video" || t == "short" {
		return id
	}
	return ""
}

// ExtractChannelID pulls the channel ID or handle from a YouTube URL.
func ExtractChannelID(rawURL string) string {
	t, id := ResolveURL(rawURL)
	if t == "channel" {
		return id
	}
	return ""
}
