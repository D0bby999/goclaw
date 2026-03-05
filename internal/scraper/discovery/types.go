package discovery

import "time"

type RobotsTxtRule struct {
	UserAgent  string
	Allow      []string
	Disallow   []string
	CrawlDelay *float64
}

type RobotsTxtRules struct {
	Rules    []RobotsTxtRule
	Sitemaps []string
}

type SitemapEntry struct {
	URL        string
	LastMod    *time.Time
	ChangeFreq string
	Priority   float64
}
