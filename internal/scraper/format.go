package scraper

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/scraper/actor"
)

const maxOutputChars = 50000

// FormatRunForLLM formats an actor.Run into a structured string for LLM consumption.
func FormatRunForLLM(run actor.Run, actorName string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Scraper: %s\n", actorName)
	fmt.Fprintf(&b, "Status: %s\n", run.Status)
	fmt.Fprintf(&b, "Items: %d scraped\n", run.Stats.ItemsScraped)
	fmt.Fprintf(&b, "Duration: %dms\n", run.Stats.DurationMs)
	fmt.Fprintf(&b, "Requests: %d total, %d failed\n", run.Stats.RequestsTotal, run.Stats.RequestsFailed)

	if len(run.Items) > 0 {
		b.WriteString("\nResults:\n")
		itemsJSON, err := json.Marshal(run.Items)
		if err != nil {
			b.WriteString("[error serializing items]\n")
		} else {
			s := string(itemsJSON)
			if len(s) > maxOutputChars {
				s = s[:maxOutputChars] + "\n... [truncated]"
			}
			b.WriteString(s)
			b.WriteByte('\n')
		}
	}

	if len(run.Errors) > 0 {
		b.WriteString("\nErrors:\n")
		for _, e := range run.Errors {
			fmt.Fprintf(&b, "- [%s] %s", e.Category, e.Message)
			if url, ok := e.Context["url"]; ok {
				fmt.Fprintf(&b, " (url: %s)", url)
			}
			b.WriteByte('\n')
		}
	}

	return b.String()
}
