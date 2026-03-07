package social

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type threadsClient struct {
	token  string
	userID string
}

func newThreadsClient(token string, metadata json.RawMessage) *threadsClient {
	c := &threadsClient{token: token}
	if metadata != nil {
		var m map[string]string
		if json.Unmarshal(metadata, &m) == nil {
			c.userID = m["threads_user_id"]
		}
	}
	return c
}

func (c *threadsClient) Platform() string { return "threads" }

func (c *threadsClient) Publish(ctx context.Context, req PublishRequest) (*PublishResult, error) {
	if c.userID == "" {
		return nil, &PlatformError{Platform: "threads", Category: ErrCategoryClient, Message: "threads_user_id required"}
	}

	// Step 1: Create container
	containerURL := fmt.Sprintf("https://graph.threads.net/v1.0/%s/threads", c.userID)
	body := map[string]any{
		"text":         req.Content,
		"media_type":   "TEXT",
		"access_token": c.token,
	}
	if len(req.Media) > 0 {
		m := req.Media[0]
		if m.MediaType == "image" {
			body["media_type"] = "IMAGE"
			body["image_url"] = m.URL
		} else {
			body["media_type"] = "VIDEO"
			body["video_url"] = m.URL
		}
	}
	if req.ReplyTo != "" {
		body["reply_to_id"] = req.ReplyTo
	}

	var containerResp struct {
		ID string `json:"id"`
	}
	if err := doJSON(ctx, "POST", containerURL, body, nil, &containerResp); err != nil {
		return nil, err
	}

	// Step 2: Publish container
	publishURL := fmt.Sprintf("https://graph.threads.net/v1.0/%s/threads_publish", c.userID)
	publishBody := map[string]any{
		"creation_id":  containerResp.ID,
		"access_token": c.token,
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := doJSON(ctx, "POST", publishURL, publishBody, nil, &resp); err != nil {
		return nil, err
	}
	return &PublishResult{
		PlatformPostID: resp.ID,
		PlatformURL:    fmt.Sprintf("https://threads.net/post/%s", resp.ID),
	}, nil
}

func (c *threadsClient) RefreshToken(ctx context.Context, _ string) (*TokenResult, error) {
	apiURL := fmt.Sprintf("https://graph.threads.net/refresh_access_token?grant_type=th_refresh_token&access_token=%s", c.token)
	var resp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := doJSON(ctx, "GET", apiURL, nil, nil, &resp); err != nil {
		return nil, err
	}
	return &TokenResult{
		AccessToken: resp.AccessToken,
		ExpiresAt:   time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second),
	}, nil
}

func (c *threadsClient) GetProfile(ctx context.Context) (*ProfileResult, error) {
	apiURL := fmt.Sprintf("https://graph.threads.net/v1.0/%s?fields=id,username,name,threads_profile_picture_url&access_token=%s", c.userID, c.token)
	var resp struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		User    string `json:"username"`
		Picture string `json:"threads_profile_picture_url"`
	}
	if err := doJSON(ctx, "GET", apiURL, nil, nil, &resp); err != nil {
		return nil, err
	}
	return &ProfileResult{ID: resp.ID, Username: resp.User, Name: resp.Name, Avatar: resp.Picture}, nil
}
