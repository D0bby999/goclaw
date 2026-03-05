package website

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/scraper/actor"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/discovery"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/queue"
)

// seedFromSitemaps fetches /sitemap.xml for each start URL domain and enqueues entries.
func (o *Orchestrator) seedFromSitemaps(ctx context.Context, q *queue.Queue, stats *actor.RunStats) {
	seen := make(map[string]bool)
	for _, u := range o.input.StartURLs {
		origin := originOf(u)
		if origin == "" || seen[origin] {
			continue
		}
		seen[origin] = true
		o.fetchAndEnqueueSitemap(ctx, origin+"/sitemap.xml", q, stats, 0)
	}
}

// fetchAndEnqueueSitemap fetches a sitemap URL, parses it, and enqueues entries.
// Recursively follows sitemap index sub-sitemaps up to depth 2.
func (o *Orchestrator) fetchAndEnqueueSitemap(ctx context.Context, sitemapURL string, q *queue.Queue, stats *actor.RunStats, depth int) {
	if depth > 2 {
		return
	}
	actor.IncrementRequests(stats)
	resp, err := o.client.Get(ctx, sitemapURL)
	if err != nil || resp.StatusCode != 200 {
		return
	}

	entries, subSitemaps, err := discovery.ParseSitemapXML(resp.Body)
	if err != nil {
		return
	}

	for _, e := range entries {
		if e.URL != "" {
			q.Add(queue.CrawlRequest{URL: discovery.NormalizeURL(e.URL), Depth: 0})
		}
	}
	for _, sub := range subSitemaps {
		o.fetchAndEnqueueSitemap(ctx, sub, q, stats, depth+1)
	}
}

// isAllowed checks robots.txt rules, fetching and caching per domain.
func (o *Orchestrator) isAllowed(ctx context.Context, rawURL string, stats *actor.RunStats) bool {
	origin := originOf(rawURL)
	if origin == "" {
		return true
	}

	rules, cached := o.robotsCache[origin]
	if !cached {
		actor.IncrementRequests(stats)
		resp, err := o.client.Get(ctx, origin+"/robots.txt")
		if err != nil || resp.StatusCode != 200 {
			o.robotsCache[origin] = nil
			return true
		}
		r := discovery.ParseRobotsTxt(resp.Body)
		o.robotsCache[origin] = &r
		rules = &r
	}
	if rules == nil {
		return true
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return true
	}
	path := u.RequestURI()
	return discovery.IsAllowed(path, *rules, crawlerUserAgent)
}

// isBanned returns true when the response signals a block or ban.
func isBanned(status int, body string) bool {
	if status == 403 || status == 429 {
		return true
	}
	lower := strings.ToLower(body)
	return strings.Contains(lower, "cloudflare") && strings.Contains(lower, "ray id")
}

// isHTML returns true when the content-type indicates an HTML document.
func isHTML(ct string) bool {
	return strings.Contains(strings.ToLower(ct), "text/html")
}

// originOf returns "scheme://host" from a URL, or empty string on error.
func originOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return ""
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host)
}

// hostOf returns the lowercased hostname from a URL.
func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// compilePatterns compiles a slice of regex strings.
func compilePatterns(patterns []string) ([]*regexp.Regexp, error) {
	result := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", p, err)
		}
		result = append(result, re)
	}
	return result, nil
}
