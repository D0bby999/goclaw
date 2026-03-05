package discovery

import (
	"encoding/xml"
	"strings"
	"time"
)

type xmlURLSet struct {
	XMLName xml.Name `xml:"urlset"`
	URLs    []xmlURL `xml:"url"`
}

type xmlURL struct {
	Loc        string  `xml:"loc"`
	LastMod    string  `xml:"lastmod"`
	ChangeFreq string  `xml:"changefreq"`
	Priority   float64 `xml:"priority"`
}

type xmlSitemapIndex struct {
	XMLName  xml.Name     `xml:"sitemapindex"`
	Sitemaps []xmlSitemap `xml:"sitemap"`
}

type xmlSitemap struct {
	Loc string `xml:"loc"`
}

// ParseSitemapXML parses sitemap XML content.
// Returns (entries, subSitemapURLs, error).
// For sitemapindex documents, entries will be empty and subSitemapURLs populated.
// For urlset documents, entries are populated and subSitemapURLs will be empty.
func ParseSitemapXML(content string) ([]SitemapEntry, []string, error) {
	content = strings.TrimSpace(content)

	// Detect sitemapindex vs urlset by peeking at the root element.
	if isSitemapIndex(content) {
		return parseSitemapIndex(content)
	}
	return parseURLSet(content)
}

func isSitemapIndex(content string) bool {
	idx := strings.Index(strings.ToLower(content), "sitemapindex")
	return idx >= 0 && idx < 500
}

func parseSitemapIndex(content string) ([]SitemapEntry, []string, error) {
	var index xmlSitemapIndex
	if err := xml.Unmarshal([]byte(content), &index); err != nil {
		return nil, nil, err
	}

	var sitemaps []string
	for _, s := range index.Sitemaps {
		if s.Loc != "" {
			sitemaps = append(sitemaps, s.Loc)
		}
	}
	return nil, sitemaps, nil
}

func parseURLSet(content string) ([]SitemapEntry, []string, error) {
	var urlset xmlURLSet
	if err := xml.Unmarshal([]byte(content), &urlset); err != nil {
		return nil, nil, err
	}

	entries := make([]SitemapEntry, 0, len(urlset.URLs))
	for _, u := range urlset.URLs {
		entry := SitemapEntry{
			URL:        u.Loc,
			ChangeFreq: u.ChangeFreq,
			Priority:   u.Priority,
		}
		if u.LastMod != "" {
			if t, err := time.Parse("2006-01-02", u.LastMod); err == nil {
				entry.LastMod = &t
			} else if t, err := time.Parse(time.RFC3339, u.LastMod); err == nil {
				entry.LastMod = &t
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil, nil
}
