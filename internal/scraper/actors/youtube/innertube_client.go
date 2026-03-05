package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nextlevelbuilder/goclaw/internal/scraper/httpclient"
	"github.com/tidwall/gjson"
)

const (
	innertubeAPIKey = "AIzaSyAO_FJ2SlqU8Q4STEHLGCilw_Y9_11qcW8"
	innertubeBase   = "https://www.youtube.com/youtubei/v1"
)

func innertubeContext() map[string]any {
	return map[string]any{
		"context": map[string]any{
			"client": map[string]any{
				"clientName":    "WEB",
				"clientVersion": "2.20240101.00.00",
			},
		},
	}
}

func postInnertube(ctx context.Context, client *httpclient.Client, endpoint string, body map[string]any) (string, error) {
	rawURL := fmt.Sprintf("%s/%s?key=%s", innertubeBase, endpoint, innertubeAPIKey)
	b, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(b))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Body, nil
}

// browseChannel fetches channel info and video list via the browse endpoint.
func browseChannel(ctx context.Context, client *httpclient.Client, channelID string) (YouTubeChannel, []YouTubeVideo, error) {
	body := innertubeContext()
	body["browseId"] = channelID

	raw, err := postInnertube(ctx, client, "browse", body)
	if err != nil {
		return YouTubeChannel{}, nil, err
	}

	ch := YouTubeChannel{
		ID:   channelID,
		Name: gjson.Get(raw, "header.c4TabbedHeaderRenderer.title").String(),
		URL:  "https://www.youtube.com/channel/" + channelID,
	}
	ch.ThumbnailURL = gjson.Get(raw, "header.c4TabbedHeaderRenderer.avatar.thumbnails.0.url").String()
	ch.Description = gjson.Get(raw, "metadata.channelMetadataRenderer.description").String()
	ch.SubscriberCount = parseViewCount(gjson.Get(raw, "header.c4TabbedHeaderRenderer.subscriberCountText.simpleText").String())

	var videos []YouTubeVideo
	gjson.Get(raw, "contents").ForEach(func(_, v gjson.Result) bool {
		extractVideosFromResult(v.Raw, channelID, ch.Name, ch.URL, &videos)
		return true
	})

	return ch, videos, nil
}

// getVideoInfo fetches detailed video info via the player endpoint.
func getVideoInfo(ctx context.Context, client *httpclient.Client, videoID string) (YouTubeVideo, error) {
	body := innertubeContext()
	body["videoId"] = videoID

	raw, err := postInnertube(ctx, client, "player", body)
	if err != nil {
		return YouTubeVideo{}, err
	}

	details := gjson.Get(raw, "videoDetails")
	video := YouTubeVideo{
		ID:           videoID,
		Title:        details.Get("title").String(),
		URL:          "https://www.youtube.com/watch?v=" + videoID,
		Description:  details.Get("shortDescription").String(),
		ChannelName:  details.Get("author").String(),
		ChannelID:    details.Get("channelId").String(),
		Duration:     details.Get("lengthSeconds").String(),
		ViewCount:    int(details.Get("viewCount").Int()),
		ThumbnailURL: details.Get("thumbnail.thumbnails.0.url").String(),
	}
	if video.ChannelID != "" {
		video.ChannelURL = "https://www.youtube.com/channel/" + video.ChannelID
	}
	return video, nil
}

// searchVideos searches YouTube for a query and returns up to maxResults videos.
func searchVideos(ctx context.Context, client *httpclient.Client, query string, maxResults int) ([]YouTubeVideo, error) {
	body := innertubeContext()
	body["query"] = query

	raw, err := postInnertube(ctx, client, "search", body)
	if err != nil {
		return nil, err
	}

	var videos []YouTubeVideo
	gjson.Get(raw, "contents").ForEach(func(_, v gjson.Result) bool {
		if len(videos) >= maxResults {
			return false
		}
		extractVideosFromResult(v.Raw, "", "", "", &videos)
		return true
	})

	return videos, nil
}
