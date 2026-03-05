package website

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/scraper/actor"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/discovery"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/extractor"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/httpclient"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/queue"
)

const crawlerUserAgent = "GoCrawler/1.0"

// Orchestrator drives the BFS crawl loop.
type Orchestrator struct {
	client      *httpclient.Client
	input       WebsiteInput
	robotsCache map[string]*discovery.RobotsTxtRules
	includeRe   []*regexp.Regexp
	excludeRe   []*regexp.Regexp
}

// NewOrchestrator creates an Orchestrator and pre-compiles regex patterns.
func NewOrchestrator(input WebsiteInput, client *httpclient.Client) (*Orchestrator, error) {
	inc, err := compilePatterns(input.IncludePatterns)
	if err != nil {
		return nil, fmt.Errorf("include patterns: %w", err)
	}
	exc, err := compilePatterns(input.ExcludePatterns)
	if err != nil {
		return nil, fmt.Errorf("exclude patterns: %w", err)
	}
	return &Orchestrator{
		client:      client,
		input:       input,
		robotsCache: make(map[string]*discovery.RobotsTxtRules),
		includeRe:   inc,
		excludeRe:   exc,
	}, nil
}

// Crawl executes the BFS crawl and returns all successfully extracted pages.
func (o *Orchestrator) Crawl(ctx context.Context, stats *actor.RunStats) ([]CrawledPage, error) {
	q := queue.New(queue.BFS)

	// Optionally seed from sitemap before adding start URLs.
	if o.input.UseSitemap {
		o.seedFromSitemaps(ctx, q, stats)
	}

	// Seed with explicit start URLs at depth 0.
	for _, u := range o.input.StartURLs {
		normalized := discovery.NormalizeURL(u)
		q.Add(queue.CrawlRequest{URL: normalized, Depth: 0})
	}

	delay := time.Duration(o.input.RequestDelayMs) * time.Millisecond
	maxPages := o.input.MaxPages
	if maxPages <= 0 {
		maxPages = 100
	}

	var pages []CrawledPage

	for !q.IsEmpty() {
		if len(pages) >= maxPages {
			break
		}

		req, ok := q.Next()
		if !ok {
			break
		}

		// Robots.txt check.
		if o.input.RespectRobotsTxt && !o.isAllowed(ctx, req.URL, stats) {
			q.MarkFailed(req.UniqueKey)
			continue
		}

		actor.IncrementRequests(stats)
		resp, err := o.client.Get(ctx, req.URL)
		if err != nil {
			var errs []actor.Error
			actor.HandleURLError(err, req.URL, &errs, stats)
			q.MarkFailed(req.UniqueKey)
			continue
		}

		// Ban / block detection.
		if isBanned(resp.StatusCode, resp.Body) {
			q.MarkFailed(req.UniqueKey)
			continue
		}

		// Skip non-HTML responses.
		if !isHTML(resp.ContentType) {
			q.MarkCompleted(req.UniqueKey)
			continue
		}

		page := o.buildPage(resp.URL, resp.Body, resp.StatusCode, req.Depth)
		pages = append(pages, page)
		stats.ItemsScraped++

		// Follow links if depth budget remains.
		if req.Depth < o.input.MaxDepth {
			o.enqueueLinks(resp.Body, resp.URL, req.Depth+1, q)
		}

		q.MarkCompleted(req.UniqueKey)

		if delay > 0 {
			if err := actor.Delay(ctx, delay); err != nil {
				return pages, nil // context cancelled
			}
		}
	}

	return pages, nil
}

// buildPage assembles a CrawledPage from a fetched HTML response.
func (o *Orchestrator) buildPage(pageURL, html string, status, depth int) CrawledPage {
	content := ExtractReadableContent(html)
	if strings.EqualFold(o.input.ExtractMode, "html") {
		content = html
	}

	return CrawledPage{
		URL:        pageURL,
		Title:      ExtractTitle(html),
		Content:    content,
		Metadata:   ExtractMetadata(html),
		JSONLD:     extractor.ExtractJSONLD(html),
		StatusCode: status,
		Depth:      depth,
		CrawledAt:  time.Now().UTC(),
	}
}

// enqueueLinks extracts, filters, and enqueues links found on a page.
func (o *Orchestrator) enqueueLinks(html, pageURL string, depth int, q *queue.Queue) {
	links, err := discovery.ExtractLinks(html, pageURL)
	if err != nil {
		return
	}

	baseHost := hostOf(pageURL)
	filtered := discovery.FilterLinks(links, o.includeRe, o.excludeRe, o.input.SameDomainOnly, baseHost)

	for _, link := range filtered {
		q.Add(queue.CrawlRequest{URL: link, Depth: depth})
	}
}

