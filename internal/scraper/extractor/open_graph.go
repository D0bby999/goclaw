package extractor

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ExtractOpenGraph extracts Open Graph metadata from HTML.
// Returns nil if no OG tags are found.
func ExtractOpenGraph(html string) *OpenGraphData {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil
	}

	og := &OpenGraphData{}
	found := false

	doc.Find("meta[property]").Each(func(_ int, s *goquery.Selection) {
		prop, _ := s.Attr("property")
		if !strings.HasPrefix(prop, "og:") {
			return
		}
		content, _ := s.Attr("content")
		found = true

		switch prop {
		case "og:title":
			og.Title = content
		case "og:description":
			og.Description = content
		case "og:image":
			og.Image = content
		case "og:url":
			og.URL = content
		case "og:type":
			og.Type = content
		case "og:site_name":
			og.SiteName = content
		}
	})

	if !found {
		return nil
	}
	return og
}
