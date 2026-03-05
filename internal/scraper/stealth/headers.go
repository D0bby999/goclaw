package stealth

import "math/rand/v2"

var acceptLanguages = []string{
	"en-US,en;q=0.9",
	"en-GB,en;q=0.9",
	"en-US,en;q=0.8,fr;q=0.6",
	"en-US,en;q=0.9,es;q=0.8",
	"en-CA,en;q=0.9",
	"en-AU,en;q=0.9",
}

// BuildStealthHeaders returns a realistic set of browser-like HTTP headers
// derived from the given fingerprint's user agent.
func BuildStealthHeaders(fp Fingerprint) map[string]string {
	h := map[string]string{
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
		"Accept-Language":           acceptLanguages[rand.IntN(len(acceptLanguages))],
		"Upgrade-Insecure-Requests": "1",
		"Cache-Control":             "max-age=0",
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "none",
		"Sec-Fetch-User":            "?1",
	}

	// Add browser-specific Sec-CH-UA headers for Chromium-based UAs.
	if containsCI(fp.UserAgent, "Chrome") {
		h["Sec-CH-UA-Mobile"] = "?0"
		h["Sec-CH-UA-Platform"] = detectPlatform(fp.UserAgent)
	}

	// Firefox does not send Sec-Fetch-User or Sec-CH-UA by default.
	if containsCI(fp.UserAgent, "Firefox") {
		delete(h, "Sec-Fetch-User")
		h["Accept"] = "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"
	}

	return h
}

func detectPlatform(ua string) string {
	switch {
	case containsCI(ua, "Windows"):
		return `"Windows"`
	case containsCI(ua, "Macintosh") || containsCI(ua, "Mac OS"):
		return `"macOS"`
	case containsCI(ua, "Linux"):
		return `"Linux"`
	default:
		return `"Unknown"`
	}
}
