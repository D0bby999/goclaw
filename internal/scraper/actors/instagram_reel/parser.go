package instagram_reel

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/scraper/extractor"
	"github.com/tidwall/gjson"
)

var (
	reSharedData   = regexp.MustCompile(`window\._sharedData\s*=\s*(\{.+?\});\s*</script>`)
	reAdditional   = regexp.MustCompile(`window\.__additionalDataLoaded\s*\(\s*'[^']*'\s*,\s*(\{.+?\})\s*\)`)
)

// ParseReelPage extracts reel data from an Instagram page HTML.
func ParseReelPage(html string) (*ReelResult, error) {
	// Try window._sharedData first
	if m := reSharedData.FindStringSubmatch(html); len(m) == 2 {
		if r := parseFromSharedData(m[1]); r != nil {
			return r, nil
		}
	}

	// Try window.__additionalDataLoaded
	if m := reAdditional.FindStringSubmatch(html); len(m) == 2 {
		if r := parseFromAdditional(m[1]); r != nil {
			return r, nil
		}
	}

	// Fallback: JSON-LD structured data
	if r := parseFromJSONLD(html); r != nil {
		return r, nil
	}

	return nil, fmt.Errorf("could not extract reel data from page")
}

func parseFromSharedData(raw string) *ReelResult {
	// Navigate to media node
	media := gjson.Get(raw, "entry_data.PostPage.0.graphql.shortcode_media")
	if !media.Exists() {
		media = gjson.Get(raw, "entry_data.PostPage.0.graphql.shortcode_media")
	}
	if !media.Exists() {
		return nil
	}
	return buildReelFromMedia(media)
}

func parseFromAdditional(raw string) *ReelResult {
	media := gjson.Get(raw, "graphql.shortcode_media")
	if !media.Exists() {
		return nil
	}
	return buildReelFromMedia(media)
}

func parseFromJSONLD(html string) *ReelResult {
	items := extractor.ExtractJSONLD(html)
	for _, item := range items {
		typeVal, _ := item["@type"].(string)
		if !strings.Contains(strings.ToLower(typeVal), "video") {
			continue
		}
		r := &ReelResult{}
		if v, ok := item["name"].(string); ok {
			r.Caption = v
		}
		if v, ok := item["contentUrl"].(string); ok {
			r.VideoURL = v
		}
		if v, ok := item["thumbnailUrl"].(string); ok {
			r.ThumbnailURL = v
		}
		if v, ok := item["url"].(string); ok {
			r.URL = v
		}
		if author, ok := item["author"].(map[string]any); ok {
			if name, ok := author["name"].(string); ok {
				r.Author = name
			}
		}
		return r
	}
	return nil
}

func buildReelFromMedia(media gjson.Result) *ReelResult {
	shortcode := media.Get("shortcode").String()
	owner := media.Get("owner")
	username := owner.Get("username").String()

	r := &ReelResult{
		ID:           media.Get("id").String(),
		URL:          "https://www.instagram.com/reel/" + shortcode + "/",
		VideoURL:     media.Get("video_url").String(),
		ThumbnailURL: media.Get("thumbnail_src").String(),
		Caption:      media.Get("edge_media_to_caption.edges.0.node.text").String(),
		Likes:        int(media.Get("edge_media_preview_like.count").Int()),
		Comments:     int(media.Get("edge_media_to_comment.count").Int()),
		Views:        int(media.Get("video_view_count").Int()),
		Duration:     int(media.Get("video_duration").Int()),
		Author:       username,
		AuthorURL:    "https://www.instagram.com/" + username + "/",
	}
	// Music
	r.MusicTitle = media.Get("clips_music_attribution_info.song_name").String()
	return r
}
