package extractor

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ExtractTables extracts all HTML tables into structured Table objects.
func ExtractTables(html string) []Table {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil
	}

	var tables []Table

	doc.Find("table").Each(func(_ int, tbl *goquery.Selection) {
		headers := extractHeaders(tbl)
		rows := extractRows(tbl, headers)
		tables = append(tables, Table{
			Headers: headers,
			Rows:    rows,
		})
	})

	return tables
}

func extractHeaders(tbl *goquery.Selection) []string {
	var headers []string

	// Try <thead> first, then first <tr> with <th> elements.
	theadRow := tbl.Find("thead tr").First()
	if theadRow.Length() > 0 {
		theadRow.Find("th").Each(func(_ int, th *goquery.Selection) {
			headers = append(headers, strings.TrimSpace(th.Text()))
		})
		if len(headers) > 0 {
			return headers
		}
	}

	// Fall back to first <tr> with <th> elements.
	tbl.Find("tr").First().Find("th").Each(func(_ int, th *goquery.Selection) {
		headers = append(headers, strings.TrimSpace(th.Text()))
	})
	if len(headers) > 0 {
		return headers
	}

	// If no <th>, use first <tr> cells as headers.
	tbl.Find("tr").First().Find("td").Each(func(_ int, td *goquery.Selection) {
		headers = append(headers, strings.TrimSpace(td.Text()))
	})

	return headers
}

func extractRows(tbl *goquery.Selection, headers []string) []map[string]string {
	var rows []map[string]string
	firstSkipped := len(headers) == 0

	tbl.Find("tr").Each(func(i int, tr *goquery.Selection) {
		// Skip header row when headers came from first <tr>.
		if i == 0 && !firstSkipped {
			hasOnlyTH := tr.Find("td").Length() == 0 && tr.Find("th").Length() > 0
			if hasOnlyTH {
				return
			}
			// First row was used as headers (td-based) — skip it.
			if tr.Find("td").Length() > 0 && len(headers) > 0 {
				return
			}
		}

		cells := tr.Find("td")
		if cells.Length() == 0 {
			return
		}

		row := make(map[string]string)
		cells.Each(func(j int, td *goquery.Selection) {
			key := ""
			if j < len(headers) {
				key = headers[j]
			} else {
				key = strings.TrimSpace(td.Text())
			}
			row[key] = strings.TrimSpace(td.Text())
		})

		if len(row) > 0 {
			rows = append(rows, row)
		}
	})

	return rows
}
