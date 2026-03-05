package discovery

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// skippedSchemes contains URL schemes that should not be followed.
var skippedSchemes = map[string]bool{
	"javascript": true,
	"mailto":     true,
	"tel":        true,
	"data":       true,
}

// ExtractLinks extracts and normalizes all hyperlinks from an HTML document.
// Relative URLs are resolved against baseURL. Skips non-HTTP schemes.
// Returns a deduplicated list of normalized URLs.
func ExtractLinks(html string, baseURL string) ([]string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var links []string

	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		href = strings.TrimSpace(href)
		if href == "" || href == "#" {
			return
		}

		parsed, err := url.Parse(href)
		if err != nil {
			return
		}

		// Skip non-web schemes.
		scheme := strings.ToLower(parsed.Scheme)
		if scheme != "" && skippedSchemes[scheme] {
			return
		}

		resolved := base.ResolveReference(parsed)

		// Only keep http/https after resolution.
		resolvedScheme := strings.ToLower(resolved.Scheme)
		if resolvedScheme != "http" && resolvedScheme != "https" {
			return
		}

		normalized := NormalizeURL(resolved.String())
		if !seen[normalized] {
			seen[normalized] = true
			links = append(links, normalized)
		}
	})

	return links, nil
}

// FilterLinks applies include/exclude regex filters and optional same-domain restriction.
// - If include patterns are provided, only links matching at least one are kept.
// - Links matching any exclude pattern are removed.
// - If sameDomain is true, only links with host matching baseHost are kept.
func FilterLinks(links []string, include, exclude []*regexp.Regexp, sameDomain bool, baseHost string) []string {
	var result []string

	for _, link := range links {
		if sameDomain {
			u, err := url.Parse(link)
			if err != nil || !strings.EqualFold(u.Hostname(), baseHost) {
				continue
			}
		}

		if len(include) > 0 && !matchesAny(link, include) {
			continue
		}

		if matchesAny(link, exclude) {
			continue
		}

		result = append(result, link)
	}

	return result
}

func matchesAny(s string, patterns []*regexp.Regexp) bool {
	for _, re := range patterns {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}
