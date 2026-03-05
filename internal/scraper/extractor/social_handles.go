package extractor

import (
	"regexp"
	"strings"
)

var (
	reTwitter   = regexp.MustCompile(`(?i)@([a-zA-Z0-9_]{1,15})`)
	reEmail     = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	rePhone     = regexp.MustCompile(`[\+]?[\d][\d\s\-\(\)]{5,14}[\d]`)
	reInstagram = regexp.MustCompile(`(?i)instagram\.com/([a-zA-Z0-9_.]{1,30})`)
	reFacebook  = regexp.MustCompile(`(?i)facebook\.com/([a-zA-Z0-9_.]{1,50})`)
	reLinkedIn  = regexp.MustCompile(`(?i)linkedin\.com/(?:in|company)/([a-zA-Z0-9_\-]{1,100})`)

	// False positives to exclude from Twitter handles (common CSS/media query tokens).
	twitterFalsePositives = map[string]bool{
		"media": true, "import": true, "charset": true, "keyframes": true,
		"supports": true, "font-face": true, "page": true, "namespace": true,
	}
)

// ExtractSocialHandles finds social media handles, emails, and phone numbers in text.
// Returns nil if nothing is found.
func ExtractSocialHandles(text string) *SocialHandles {
	h := &SocialHandles{
		Twitter:   extractTwitter(text),
		Instagram: extractURLMatches(reInstagram, text),
		Facebook:  extractURLMatches(reFacebook, text),
		LinkedIn:  extractURLMatches(reLinkedIn, text),
		Emails:    dedup(reEmail.FindAllString(text, -1)),
		Phones:    extractPhones(text),
	}

	if isEmpty(h) {
		return nil
	}
	return h
}

func extractTwitter(text string) []string {
	matches := reTwitter.FindAllStringSubmatch(text, -1)
	seen := make(map[string]bool)
	var result []string
	for _, m := range matches {
		handle := strings.ToLower(m[1])
		if twitterFalsePositives[handle] {
			continue
		}
		if !seen[handle] {
			seen[handle] = true
			result = append(result, "@"+m[1])
		}
	}
	return result
}

func extractURLMatches(re *regexp.Regexp, text string) []string {
	matches := re.FindAllStringSubmatch(text, -1)
	seen := make(map[string]bool)
	var result []string
	for _, m := range matches {
		handle := m[1]
		if !seen[handle] {
			seen[handle] = true
			result = append(result, handle)
		}
	}
	return result
}

func extractPhones(text string) []string {
	matches := rePhone.FindAllString(text, -1)
	seen := make(map[string]bool)
	var result []string
	for _, m := range matches {
		normalized := strings.TrimSpace(m)
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, normalized)
		}
	}
	return result
}

func dedup(items []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range items {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func isEmpty(h *SocialHandles) bool {
	return len(h.Twitter) == 0 && len(h.Instagram) == 0 &&
		len(h.Facebook) == 0 && len(h.LinkedIn) == 0 &&
		len(h.Emails) == 0 && len(h.Phones) == 0
}
