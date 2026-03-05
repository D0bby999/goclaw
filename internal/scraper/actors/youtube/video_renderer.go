package youtube

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

// extractVideosFromResult walks nested JSON looking for videoRenderer/gridVideoRenderer.
func extractVideosFromResult(raw, channelID, channelName, channelURL string, out *[]YouTubeVideo) {
	r := gjson.Parse(raw)
	r.ForEach(func(k, v gjson.Result) bool {
		key := k.String()
		if key == "videoRenderer" || key == "gridVideoRenderer" {
			vid := parseVideoRenderer(v, channelID, channelName, channelURL)
			*out = append(*out, vid)
		} else if v.IsObject() || v.IsArray() {
			extractVideosFromResult(v.Raw, channelID, channelName, channelURL, out)
		}
		return true
	})
}

func parseVideoRenderer(v gjson.Result, channelID, channelName, channelURL string) YouTubeVideo {
	videoID := v.Get("videoId").String()
	title := extractRuns(v.Get("title.runs").Raw)
	if title == "" {
		title = v.Get("title.simpleText").String()
	}
	thumb := v.Get("thumbnail.thumbnails.0.url").String()
	viewText := v.Get("viewCountText.simpleText").String()
	if viewText == "" {
		viewText = extractRuns(v.Get("viewCountText.runs").Raw)
	}
	pubDate := v.Get("publishedTimeText.simpleText").String()
	duration := v.Get("lengthText.simpleText").String()

	chName := channelName
	chURL := channelURL
	chID := channelID
	if chName == "" {
		chName = extractRuns(v.Get("ownerText.runs").Raw)
	}
	if chID == "" {
		chID = v.Get("ownerText.runs.0.navigationEndpoint.browseEndpoint.browseId").String()
	}
	if chURL == "" && chID != "" {
		chURL = "https://www.youtube.com/channel/" + chID
	}

	return YouTubeVideo{
		ID:           videoID,
		Title:        title,
		URL:          "https://www.youtube.com/watch?v=" + videoID,
		ThumbnailURL: thumb,
		Date:         pubDate,
		Duration:     duration,
		ViewCount:    parseViewCount(viewText),
		ChannelName:  chName,
		ChannelURL:   chURL,
		ChannelID:    chID,
	}
}

// extractRuns joins runs[].text into a single string.
func extractRuns(runsRaw string) string {
	if runsRaw == "" {
		return ""
	}
	var sb strings.Builder
	gjson.Parse(runsRaw).ForEach(func(_, v gjson.Result) bool {
		sb.WriteString(v.Get("text").String())
		return true
	})
	return sb.String()
}

// parseViewCount extracts a numeric value from strings like "1.2M views".
func parseViewCount(s string) int {
	s = strings.ToLower(strings.ReplaceAll(s, ",", ""))
	var n float64
	multiplier := 1.0
	switch {
	case strings.Contains(s, "b"):
		multiplier = 1_000_000_000
	case strings.Contains(s, "m"):
		multiplier = 1_000_000
	case strings.Contains(s, "k"):
		multiplier = 1_000
	}
	clean := strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || r == '.' {
			return r
		}
		return -1
	}, s)
	fmt.Sscanf(clean, "%f", &n)
	return int(n * multiplier)
}
