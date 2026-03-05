package extractor

import (
	"encoding/json"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ExtractJSONLD extracts JSON-LD structured data from HTML.
func ExtractJSONLD(html string) []map[string]any {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil
	}

	var results []map[string]any

	doc.Find("script[type='application/ld+json']").Each(func(_ int, s *goquery.Selection) {
		raw := strings.TrimSpace(s.Text())
		if raw == "" {
			return
		}

		if strings.HasPrefix(raw, "[") {
			var arr []map[string]any
			if err := json.Unmarshal([]byte(raw), &arr); err == nil {
				results = append(results, arr...)
			}
		} else {
			var obj map[string]any
			if err := json.Unmarshal([]byte(raw), &obj); err == nil {
				results = append(results, obj)
			}
		}
	})

	return results
}
