package tiktok

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/scraper/httpclient"
	"github.com/tidwall/gjson"
)

const tikwmAPI = "https://www.tikwm.com/api"

// fetchVideo fetches a single video by its TikTok URL via tikwm.com API.
func fetchVideo(ctx context.Context, client *httpclient.Client, videoURL string) (TikTokVideo, error) {
	target := tikwmAPI + "/?url=" + url.QueryEscape(videoURL)
	resp, err := client.Get(ctx, target)
	if err != nil {
		return TikTokVideo{}, fmt.Errorf("fetch video: %w", err)
	}
	if resp.StatusCode != 200 {
		return TikTokVideo{}, fmt.Errorf("fetch video: status %d", resp.StatusCode)
	}
	if gjson.Get(resp.Body, "code").Int() != 0 {
		return TikTokVideo{}, fmt.Errorf("fetch video: %s", gjson.Get(resp.Body, "msg").String())
	}
	return parseTikwmVideo(gjson.Get(resp.Body, "data")), nil
}

// fetchUserInfo fetches a user profile via tikwm.com API.
func fetchUserInfo(ctx context.Context, client *httpclient.Client, username string) (TikTokProfile, error) {
	target := tikwmAPI + "/user/info?unique_id=" + url.QueryEscape(username)
	resp, err := client.Get(ctx, target)
	if err != nil {
		return TikTokProfile{}, fmt.Errorf("user info %s: %w", username, err)
	}
	if resp.StatusCode != 200 {
		return TikTokProfile{}, fmt.Errorf("user info %s: status %d", username, resp.StatusCode)
	}
	if gjson.Get(resp.Body, "code").Int() != 0 {
		return TikTokProfile{}, fmt.Errorf("user info %s: %s", username, gjson.Get(resp.Body, "msg").String())
	}
	d := gjson.Get(resp.Body, "data")
	user := d.Get("user")
	stats := d.Get("stats")
	return TikTokProfile{
		Username:    user.Get("uniqueId").String(),
		DisplayName: user.Get("nickname").String(),
		Bio:         user.Get("signature").String(),
		URL:         "https://www.tiktok.com/@" + user.Get("uniqueId").String(),
		Followers:   int(stats.Get("followerCount").Int()),
		Following:   int(stats.Get("followingCount").Int()),
		Likes:       int(stats.Get("heartCount").Int()),
		VideoCount:  int(stats.Get("videoCount").Int()),
		AvatarURL:   user.Get("avatarLarger").String(),
		Verified:    user.Get("verified").Bool(),
	}, nil
}

// searchVideos searches TikTok videos via tikwm.com API.
func searchVideos(ctx context.Context, client *httpclient.Client, query string, count int) ([]TikTokVideo, error) {
	target := tikwmAPI + "/feed/search?keywords=" + url.QueryEscape(query) + "&count=" + strconv.Itoa(count)
	resp, err := client.Get(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("search tiktok: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("search tiktok: status %d", resp.StatusCode)
	}
	if gjson.Get(resp.Body, "code").Int() != 0 {
		return nil, fmt.Errorf("search tiktok: %s", gjson.Get(resp.Body, "msg").String())
	}
	var videos []TikTokVideo
	gjson.Get(resp.Body, "data.videos").ForEach(func(_, v gjson.Result) bool {
		videos = append(videos, parseTikwmSearchVideo(v))
		return true
	})
	return videos, nil
}

// parseTikwmVideo converts a tikwm.com video detail response to TikTokVideo.
func parseTikwmVideo(d gjson.Result) TikTokVideo {
	author := d.Get("author.unique_id").String()
	videoID := d.Get("id").String()
	videoURL := fmt.Sprintf("https://www.tiktok.com/@%s/video/%s", author, videoID)
	return TikTokVideo{
		ID:          videoID,
		URL:         videoURL,
		Title:       d.Get("title").String(),
		Author:      author,
		AuthorURL:   "https://www.tiktok.com/@" + author,
		Likes:       int(d.Get("digg_count").Int()),
		Comments:    int(d.Get("comment_count").Int()),
		Shares:      int(d.Get("share_count").Int()),
		Views:       int(d.Get("play_count").Int()),
		Duration:    int(d.Get("duration").Int()),
		CreatedAt:   time.Unix(d.Get("create_time").Int(), 0).UTC().Format(time.RFC3339),
		MusicTitle:  d.Get("music_info.title").String(),
		MusicAuthor: d.Get("music_info.author").String(),
		CoverURL:    d.Get("cover").String(),
		VideoURL:    d.Get("play").String(),
		VideoHDURL:  d.Get("hdplay").String(),
	}
}

// parseTikwmSearchVideo converts a tikwm.com search result to TikTokVideo.
func parseTikwmSearchVideo(v gjson.Result) TikTokVideo {
	author := v.Get("author.unique_id").String()
	videoID := v.Get("video_id").String()
	videoURL := fmt.Sprintf("https://www.tiktok.com/@%s/video/%s", author, videoID)
	return TikTokVideo{
		ID:          videoID,
		URL:         videoURL,
		Title:       v.Get("title").String(),
		Author:      author,
		AuthorURL:   "https://www.tiktok.com/@" + author,
		Likes:       int(v.Get("digg_count").Int()),
		Comments:    int(v.Get("comment_count").Int()),
		Shares:      int(v.Get("share_count").Int()),
		Views:       int(v.Get("play_count").Int()),
		Duration:    int(v.Get("duration").Int()),
		CreatedAt:   time.Unix(v.Get("create_time").Int(), 0).UTC().Format(time.RFC3339),
		MusicTitle:  v.Get("music_info.title").String(),
		MusicAuthor: v.Get("music_info.author").String(),
		CoverURL:    v.Get("cover").String(),
		VideoURL:    v.Get("play").String(),
		VideoHDURL:  v.Get("hdplay").String(),
	}
}
