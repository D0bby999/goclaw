package website

import "time"

// WebsiteInput defines the configuration for a website crawl run.
type WebsiteInput struct {
	StartURLs        []string `json:"start_urls"`
	MaxPages         int      `json:"max_pages"`
	MaxDepth         int      `json:"max_depth"`
	IncludePatterns  []string `json:"include_patterns,omitempty"`
	ExcludePatterns  []string `json:"exclude_patterns,omitempty"`
	SameDomainOnly   bool     `json:"same_domain_only"`
	RespectRobotsTxt bool     `json:"respect_robots_txt"`
	UseSitemap       bool     `json:"use_sitemap"`
	ExtractMode      string   `json:"extract_mode"` // "text" or "html" (default: "text")
	RequestDelayMs   int      `json:"request_delay_ms"`
}

// CrawledPage holds the extracted data for a single crawled page.
type CrawledPage struct {
	URL        string            `json:"url"`
	Title      string            `json:"title"`
	Content    string            `json:"content"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	JSONLD     []map[string]any  `json:"json_ld,omitempty"`
	StatusCode int               `json:"status_code"`
	Depth      int               `json:"depth"`
	CrawledAt  time.Time         `json:"crawled_at"`
}
