package google_trends

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/scraper/httpclient"
	"github.com/tidwall/gjson"
)

const (
	trendsExploreURL = "https://trends.google.com/trends/api/explore"
	trendsMultiline  = "https://trends.google.com/trends/api/widgetdata/multiline"
	trendsRelated    = "https://trends.google.com/trends/api/widgetdata/relatedsearches"
	trendsDailyRSS   = "https://trends.google.com/trending/rss"
	trendsHomePage   = "https://trends.google.com/trends"
)

// stripPrefix removes the )]}'\n anti-JSON hijacking prefix Google adds.
func stripPrefix(body string) string {
	if idx := strings.Index(body, "\n"); idx >= 0 {
		return body[idx+1:]
	}
	return body
}

// initSession fetches the trends page to obtain a NID cookie.
func initSession(ctx context.Context, client *httpclient.Client) (string, error) {
	resp, err := client.Get(ctx, trendsHomePage)
	if err != nil {
		return "", fmt.Errorf("init session: %w", err)
	}
	// Extract NID from Set-Cookie header
	for _, cookie := range resp.Headers["Set-Cookie"] {
		if strings.HasPrefix(cookie, "NID=") {
			parts := strings.SplitN(cookie, ";", 2)
			return parts[0], nil
		}
	}
	return "", nil
}

// widgetInfo holds a token and its full request payload from the explore response.
type widgetInfo struct {
	Token   string
	Request string
}

// fetchExplore retrieves widget tokens for the given keywords/geo/timeRange.
func fetchExplore(ctx context.Context, client *httpclient.Client, keywords []string, geo, timeRange, cookie string) (map[string]widgetInfo, error) {
	compList := make([]map[string]any, len(keywords))
	for i, kw := range keywords {
		compList[i] = map[string]any{
			"keyword": kw,
			"geo":     geo,
			"time":    timeRange,
		}
	}
	reqJSON, _ := json.Marshal(map[string]any{"comparisonItem": compList, "category": 0, "property": ""})

	params := url.Values{}
	params.Set("hl", "en-US")
	params.Set("tz", "-420")
	params.Set("req", string(reqJSON))

	reqURL := trendsExploreURL + "?" + params.Encode()
	resp, err := doWithCookie(ctx, client, reqURL, cookie)
	if err != nil {
		return nil, fmt.Errorf("fetch explore: %w", err)
	}

	clean := stripPrefix(resp)
	widgets := map[string]widgetInfo{}
	gjson.Get(clean, "widgets").ForEach(func(_, w gjson.Result) bool {
		id := w.Get("id").String()
		token := w.Get("token").String()
		reqObj := w.Get("request").Raw
		if token != "" {
			widgets[id] = widgetInfo{Token: token, Request: reqObj}
		}
		return true
	})
	return widgets, nil
}

// fetchInterestOverTime retrieves time-series interest data.
func fetchInterestOverTime(ctx context.Context, client *httpclient.Client, wi widgetInfo, cookie string) ([]InterestPoint, error) {
	params := url.Values{}
	params.Set("hl", "en-US")
	params.Set("tz", "-420")
	params.Set("token", wi.Token)
	params.Set("req", wi.Request)

	resp, err := doWithCookie(ctx, client, trendsMultiline+"?"+params.Encode(), cookie)
	if err != nil {
		return nil, fmt.Errorf("fetch interest over time: %w", err)
	}

	clean := stripPrefix(resp)
	var points []InterestPoint
	gjson.Get(clean, "default.timelineData").ForEach(func(_, v gjson.Result) bool {
		points = append(points, InterestPoint{
			Time:  v.Get("formattedTime").String(),
			Value: int(v.Get("value.0").Int()),
		})
		return true
	})
	return points, nil
}

// fetchRelatedQueries retrieves related and rising queries.
func fetchRelatedQueries(ctx context.Context, client *httpclient.Client, wi widgetInfo, cookie string) ([]RelatedItem, error) {
	params := url.Values{}
	params.Set("hl", "en-US")
	params.Set("tz", "-420")
	params.Set("token", wi.Token)
	params.Set("req", wi.Request)

	resp, err := doWithCookie(ctx, client, trendsRelated+"?"+params.Encode(), cookie)
	if err != nil {
		return nil, fmt.Errorf("fetch related queries: %w", err)
	}

	clean := stripPrefix(resp)
	var items []RelatedItem

	gjson.Get(clean, "default.rankedList.0.rankedKeyword").ForEach(func(_, v gjson.Result) bool {
		items = append(items, RelatedItem{
			Query: v.Get("query").String(),
			Value: int(v.Get("value").Int()),
			Type:  "top",
		})
		return true
	})
	gjson.Get(clean, "default.rankedList.1.rankedKeyword").ForEach(func(_, v gjson.Result) bool {
		items = append(items, RelatedItem{
			Query: v.Get("query").String(),
			Value: int(v.Get("value").Int()),
			Type:  "rising",
		})
		return true
	})
	return items, nil
}

// fetchDailyTrending retrieves daily trending searches via RSS feed.
func fetchDailyTrending(ctx context.Context, client *httpclient.Client, geo, _ string) ([]TrendingItem, error) {
	params := url.Values{}
	params.Set("geo", geo)

	resp, err := doWithCookie(ctx, client, trendsDailyRSS+"?"+params.Encode(), "")
	if err != nil {
		return nil, fmt.Errorf("fetch daily trending: %w", err)
	}

	return parseTrendingRSS(resp), nil
}

// parseTrendingRSS extracts trending items from Google Trends RSS XML.
func parseTrendingRSS(body string) []TrendingItem {
	var items []TrendingItem
	// Simple XML parsing — split by <item> blocks.
	parts := strings.Split(body, "<item>")
	for _, part := range parts[1:] { // skip header
		end := strings.Index(part, "</item>")
		if end < 0 {
			continue
		}
		block := part[:end]
		item := TrendingItem{
			Title:         extractXMLTag(block, "title"),
			TrafficVolume: extractXMLTag(block, "ht:approx_traffic"),
		}
		// First news item
		if newsStart := strings.Index(block, "<ht:news_item>"); newsStart >= 0 {
			newsEnd := strings.Index(block[newsStart:], "</ht:news_item>")
			if newsEnd > 0 {
				news := block[newsStart : newsStart+newsEnd]
				item.Summary = extractXMLTag(news, "ht:news_item_title")
				item.Source = extractXMLTag(news, "ht:news_item_source")
				item.URL = extractXMLTag(news, "ht:news_item_url")
			}
		}
		if item.Title != "" {
			items = append(items, item)
		}
	}
	return items
}

func extractXMLTag(block, tag string) string {
	open := "<" + tag + ">"
	close := "</" + tag + ">"
	start := strings.Index(block, open)
	if start < 0 {
		return ""
	}
	start += len(open)
	end := strings.Index(block[start:], close)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(block[start : start+end])
}

func doWithCookie(ctx context.Context, client *httpclient.Client, reqURL, cookie string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", err
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := client.Do(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Body, nil
}
