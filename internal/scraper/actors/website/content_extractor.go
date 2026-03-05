package website

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/extractor"
)

var multiSpaceRe = regexp.MustCompile(`\s{2,}`)

// ExtractReadableContent extracts clean readable text from HTML.
// Prefers <article>, <main>, or [role="main"]; falls back to <body>.
// Strips nav, footer, header, aside, script, style, noscript before extracting text.
func ExtractReadableContent(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}

	// Remove noise elements everywhere in the doc first.
	doc.Find("nav, footer, header, aside, script, style, noscript").Remove()

	// Pick the best content subtree.
	var sel *goquery.Selection
	if s := doc.Find("article").First(); s.Length() > 0 {
		sel = s
	} else if s := doc.Find("main").First(); s.Length() > 0 {
		sel = s
	} else if s := doc.Find("[role='main']").First(); s.Length() > 0 {
		sel = s
	} else {
		sel = doc.Find("body")
	}

	text := sel.Text()
	text = multiSpaceRe.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// ExtractTitle returns the page <title> text, falling back to the first <h1>.
func ExtractTitle(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}

	if title := strings.TrimSpace(doc.Find("title").First().Text()); title != "" {
		return title
	}
	return strings.TrimSpace(doc.Find("h1").First().Text())
}

// ExtractMetadata returns Open Graph fields as a flat string map.
func ExtractMetadata(html string) map[string]string {
	og := extractor.ExtractOpenGraph(html)
	if og == nil {
		return nil
	}

	m := make(map[string]string, 6)
	if og.Title != "" {
		m["og:title"] = og.Title
	}
	if og.Description != "" {
		m["og:description"] = og.Description
	}
	if og.Image != "" {
		m["og:image"] = og.Image
	}
	if og.URL != "" {
		m["og:url"] = og.URL
	}
	if og.Type != "" {
		m["og:type"] = og.Type
	}
	if og.SiteName != "" {
		m["og:site_name"] = og.SiteName
	}

	if len(m) == 0 {
		return nil
	}
	return m
}
