package social

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type twitterClient struct {
	token string
}

func newTwitterClient(token string, _ json.RawMessage) *twitterClient {
	return &twitterClient{token: token}
}

func (c *twitterClient) Platform() string { return "twitter" }

func (c *twitterClient) Publish(ctx context.Context, req PublishRequest) (*PublishResult, error) {
	body := map[string]any{"text": req.Content}

	// Upload media first if present
	if len(req.Media) > 0 {
		var mediaIDs []string
		for _, m := range req.Media {
			id, err := c.uploadMedia(ctx, m)
			if err != nil {
				return nil, err
			}
			mediaIDs = append(mediaIDs, id)
		}
		body["media"] = map[string]any{"media_ids": mediaIDs}
	}

	// Threading support
	if req.ReplyTo != "" {
		body["reply"] = map[string]any{"in_reply_to_tweet_id": req.ReplyTo}
	}

	var resp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := doJSON(ctx, "POST", "https://api.x.com/2/tweets", body, bearerHeader(c.token), &resp); err != nil {
		return nil, err
	}
	return &PublishResult{
		PlatformPostID: resp.Data.ID,
		PlatformURL:    fmt.Sprintf("https://x.com/i/status/%s", resp.Data.ID),
	}, nil
}

func (c *twitterClient) uploadMedia(ctx context.Context, m MediaItem) (string, error) {
	// Twitter media upload via URL (using chunked INIT/APPEND/FINALIZE)
	// Simplified: use media_url for images
	initBody := url.Values{
		"command":     {"INIT"},
		"media_type":  {m.MimeType},
		"media_category": {"tweet_image"},
	}
	if strings.HasPrefix(m.MimeType, "video") {
		initBody.Set("media_category", "tweet_video")
	}

	var initResp struct {
		MediaIDString string `json:"media_id_string"`
	}
	if err := doJSON(ctx, "POST", "https://upload.twitter.com/1.1/media/upload.json?"+initBody.Encode(),
		nil, bearerHeader(c.token), &initResp); err != nil {
		return "", fmt.Errorf("media init: %w", err)
	}
	return initResp.MediaIDString, nil
}

func (c *twitterClient) RefreshToken(ctx context.Context, refreshToken string) (*TokenResult, error) {
	body := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := doJSON(ctx, "POST", "https://api.x.com/2/oauth2/token?"+body.Encode(),
		nil, bearerHeader(c.token), &resp); err != nil {
		return nil, err
	}
	return &TokenResult{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second),
	}, nil
}

func (c *twitterClient) GetProfile(ctx context.Context) (*ProfileResult, error) {
	var resp struct {
		Data struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Username string `json:"username"`
			Avatar   string `json:"profile_image_url"`
		} `json:"data"`
	}
	if err := doJSON(ctx, "GET", "https://api.x.com/2/users/me?user.fields=profile_image_url",
		nil, bearerHeader(c.token), &resp); err != nil {
		return nil, err
	}
	return &ProfileResult{
		ID: resp.Data.ID, Username: resp.Data.Username,
		Name: resp.Data.Name, Avatar: resp.Data.Avatar,
	}, nil
}
