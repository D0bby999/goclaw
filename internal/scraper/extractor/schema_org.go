package extractor

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ExtractSchemaOrg extracts Schema.org microdata from HTML elements with itemscope.
func ExtractSchemaOrg(html string) []map[string]any {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil
	}

	var results []map[string]any

	doc.Find("[itemscope]").Each(func(_ int, s *goquery.Selection) {
		item := make(map[string]any)

		if itemtype, exists := s.Attr("itemtype"); exists && itemtype != "" {
			item["itemtype"] = itemtype
		}

		s.Find("[itemprop]").Each(func(_ int, prop *goquery.Selection) {
			name, _ := prop.Attr("itemprop")
			if name == "" {
				return
			}
			text := strings.TrimSpace(prop.Text())
			if existing, ok := item[name]; ok {
				switch v := existing.(type) {
				case []string:
					item[name] = append(v, text)
				case string:
					item[name] = []string{v, text}
				}
			} else {
				item[name] = text
			}
		})

		if len(item) > 0 {
			results = append(results, item)
		}
	})

	return results
}
