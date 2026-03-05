package extractor

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ExtractByCSS returns the text content of all elements matching the CSS selector.
func ExtractByCSS(html, selector string) ([]string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	var results []string
	doc.Find(selector).Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		results = append(results, text)
	})

	return results, nil
}

// ExtractAttrByCSS returns the attribute values of all elements matching the CSS selector.
func ExtractAttrByCSS(html, selector, attr string) ([]string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	var results []string
	doc.Find(selector).Each(func(_ int, s *goquery.Selection) {
		if val, exists := s.Attr(attr); exists {
			results = append(results, val)
		}
	})

	return results, nil
}
