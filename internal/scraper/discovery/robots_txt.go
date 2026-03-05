package discovery

import (
	"bufio"
	"regexp"
	"strconv"
	"strings"
)

// ParseRobotsTxt parses robots.txt content into structured rules.
func ParseRobotsTxt(content string) RobotsTxtRules {
	var result RobotsTxtRules
	var current *RobotsTxtRule

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Strip inline comments.
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		directive := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch strings.ToLower(directive) {
		case "user-agent":
			if current != nil {
				result.Rules = append(result.Rules, *current)
			}
			current = &RobotsTxtRule{UserAgent: value}

		case "allow":
			if current != nil {
				current.Allow = append(current.Allow, value)
			}

		case "disallow":
			if current != nil {
				current.Disallow = append(current.Disallow, value)
			}

		case "crawl-delay":
			if current != nil {
				if delay, err := strconv.ParseFloat(value, 64); err == nil {
					current.CrawlDelay = &delay
				}
			}

		case "sitemap":
			result.Sitemaps = append(result.Sitemaps, value)
		}
	}

	if current != nil {
		result.Rules = append(result.Rules, *current)
	}

	return result
}

// IsAllowed checks whether a URL path is allowed for a given user agent.
// Allow rules take precedence over Disallow when both match.
func IsAllowed(urlPath string, rules RobotsTxtRules, userAgent string) bool {
	var matched *RobotsTxtRule
	var wildcard *RobotsTxtRule

	for i := range rules.Rules {
		r := &rules.Rules[i]
		if strings.EqualFold(r.UserAgent, userAgent) {
			matched = r
			break
		}
		if r.UserAgent == "*" {
			wildcard = r
		}
	}

	if matched == nil {
		matched = wildcard
	}
	if matched == nil {
		return true // No applicable rule — allow by default.
	}

	// Find the most specific matching allow and disallow rules.
	allowLen := -1
	disallowLen := -1

	for _, pattern := range matched.Allow {
		if matchPath(pattern, urlPath) {
			if len(pattern) > allowLen {
				allowLen = len(pattern)
			}
		}
	}

	for _, pattern := range matched.Disallow {
		if pattern == "" {
			continue // Empty Disallow means allow all.
		}
		if matchPath(pattern, urlPath) {
			if len(pattern) > disallowLen {
				disallowLen = len(pattern)
			}
		}
	}

	if disallowLen < 0 {
		return true // No matching disallow rule.
	}
	return allowLen >= disallowLen // Allow wins on tie or longer match.
}

// matchPath checks if a robots.txt path pattern matches the given URL path.
// Supports * wildcard and $ end-of-string anchor.
func matchPath(pattern, path string) bool {
	if pattern == "/" {
		return strings.HasPrefix(path, "/")
	}

	// Convert pattern to regex: escape, replace * with .*, handle $ anchor.
	regexStr := regexp.QuoteMeta(pattern)
	regexStr = strings.ReplaceAll(regexStr, `\*`, `.*`)
	if strings.HasSuffix(regexStr, `\$`) {
		regexStr = strings.TrimSuffix(regexStr, `\$`) + "$"
	}

	re, err := regexp.Compile("^" + regexStr)
	if err != nil {
		return strings.HasPrefix(path, pattern)
	}
	return re.MatchString(path)
}
