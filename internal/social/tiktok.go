package social

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type tiktokClient struct {
	token  string
	openID string
}

func newTikTokClient(token string, metadata json.RawMessage) *tiktokClient {
	c := &tiktokClient{token: token}
	if metadata != nil {
		var m map[string]string
		if json.Unmarshal(metadata, &m) == nil {
			c.openID = m["open_id"]
		}
	}
	return c
}

func (c *tiktokClient) Platform() string { return "tiktok" }

func (c *tiktokClient) Publish(ctx context.Context, req PublishRequest) (*PublishResult, error) {
	if len(req.Media) == 0 {
		return nil, &PlatformError{Platform: "tiktok", Category: ErrCategoryClient, Message: "media required"}
	}

	// TikTok Content Posting API: direct post or photo post
	body := map[string]any{
		"post_info": map[string]any{
			"title":        req.Content,
			"privacy_level": "SELF_ONLY",
		},
		"source_info": map[string]any{
			"source":       "PULL_FROM_URL",
			"video_url":    req.Media[0].URL,
		},
	}

	if req.Media[0].MediaType == "image" {
		body["source_info"] = map[string]any{
			"source":    "PULL_FROM_URL",
			"photo_urls": []string{req.Media[0].URL},
		}
		body["media_type"] = "PHOTO"
	}

	var resp struct {
		Data struct {
			PublishID string `json:"publish_id"`
		} `json:"data"`
	}
	if err := doJSON(ctx, "POST", "https://open.tiktokapis.com/v2/post/publish/video/init/",
		body, bearerHeader(c.token), &resp); err != nil {
		return nil, err
	}
	return &PublishResult{
		PlatformPostID: resp.Data.PublishID,
		PlatformURL:    fmt.Sprintf("https://tiktok.com/@%s", c.openID),
	}, nil
}

func (c *tiktokClient) RefreshToken(ctx context.Context, refreshToken string) (*TokenResult, error) {
	body := map[string]any{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
	}
	var resp struct {
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int    `json:"expires_in"`
		} `json:"data"`
	}
	if err := doJSON(ctx, "POST", "https://open.tiktokapis.com/v2/oauth/token/",
		body, nil, &resp); err != nil {
		return nil, err
	}
	return &TokenResult{
		AccessToken:  resp.Data.AccessToken,
		RefreshToken: resp.Data.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(resp.Data.ExpiresIn) * time.Second),
	}, nil
}

func (c *tiktokClient) GetProfile(ctx context.Context) (*ProfileResult, error) {
	body := map[string]any{"fields": []string{"open_id", "display_name", "avatar_url"}}
	var resp struct {
		Data struct {
			User struct {
				OpenID      string `json:"open_id"`
				DisplayName string `json:"display_name"`
				AvatarURL   string `json:"avatar_url"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := doJSON(ctx, "POST", "https://open.tiktokapis.com/v2/user/info/",
		body, bearerHeader(c.token), &resp); err != nil {
		return nil, err
	}
	u := resp.Data.User
	return &ProfileResult{ID: u.OpenID, Name: u.DisplayName, Avatar: u.AvatarURL}, nil
}
