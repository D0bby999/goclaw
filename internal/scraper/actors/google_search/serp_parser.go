package google_search

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ParseGoogleSERP extracts structured search results from Google HTML.
func ParseGoogleSERP(html, query string) GoogleSearchResult {
	result := GoogleSearchResult{Query: query}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return result
	}

	pos := 1

	// Strategy 1: modern Google SERP uses div.yuRUbf for each result link container.
	doc.Find("div.yuRUbf").Each(func(_ int, s *goquery.Selection) {
		a := s.Find("a").First()
		href, exists := a.Attr("href")
		if !exists || !strings.HasPrefix(href, "http") {
			return
		}
		title := strings.TrimSpace(a.Find("h3").First().Text())
		if title == "" {
			return
		}

		// Snippet is in a sibling or parent's sibling with class VwiC3b or data-sncf.
		snippet := ""
		parent := s.Parent()
		parent.Find(".VwiC3b").Each(func(_ int, snip *goquery.Selection) {
			if snippet == "" {
				snippet = strings.TrimSpace(snip.Text())
			}
		})
		if snippet == "" {
			parent.Find("[data-sncf]").Each(func(_ int, snip *goquery.Selection) {
				if snippet == "" {
					snippet = strings.TrimSpace(snip.Text())
				}
			})
		}

		result.OrganicResults = append(result.OrganicResults, OrganicResult{
			Position: pos,
			Title:    title,
			URL:      href,
			Snippet:  snippet,
		})
		pos++
	})

	// Strategy 2 (fallback): older Google SERP with div.g containers.
	if len(result.OrganicResults) == 0 {
		doc.Find("div.g").Each(func(_ int, s *goquery.Selection) {
			title := strings.TrimSpace(s.Find("h3").First().Text())
			if title == "" {
				return
			}
			href := ""
			s.Find("a").Each(func(_ int, a *goquery.Selection) {
				if href != "" {
					return
				}
				if h, exists := a.Attr("href"); exists && strings.HasPrefix(h, "http") {
					href = h
				}
			})
			if href == "" {
				return
			}

			snippet := ""
			s.Find(".VwiC3b").Each(func(_ int, snip *goquery.Selection) {
				if snippet == "" {
					snippet = strings.TrimSpace(snip.Text())
				}
			})

			result.OrganicResults = append(result.OrganicResults, OrganicResult{
				Position: pos,
				Title:    title,
				URL:      href,
				Snippet:  snippet,
			})
			pos++
		})
	}

	// People Also Ask
	doc.Find("div[data-q]").Each(func(_ int, s *goquery.Selection) {
		if q, exists := s.Attr("data-q"); exists && q != "" {
			result.PeopleAlsoAsk = append(result.PeopleAlsoAsk, q)
		}
	})

	// Related queries
	doc.Find("a.k8XOCe, div.s75CSd a").Each(func(_ int, s *goquery.Selection) {
		if t := strings.TrimSpace(s.Text()); t != "" {
			result.RelatedQueries = append(result.RelatedQueries, t)
		}
	})

	// Next page
	doc.Find("a#pnnext, a[aria-label='Next']").Each(func(_ int, s *goquery.Selection) {
		result.HasNextPage = true
	})

	return result
}
