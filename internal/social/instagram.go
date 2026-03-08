package social

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type instagramClient struct {
	token string
	igID  string // Instagram Business Account ID
}

func newInstagramClient(token string, metadata json.RawMessage) *instagramClient {
	c := &instagramClient{token: token}
	if metadata != nil {
		var m map[string]string
		if json.Unmarshal(metadata, &m) == nil {
			c.igID = m["ig_user_id"]
		}
	}
	return c
}

func (c *instagramClient) Platform() string { return "instagram" }

func (c *instagramClient) Publish(ctx context.Context, req PublishRequest) (*PublishResult, error) {
	if c.igID == "" {
		return nil, &PlatformError{Platform: "instagram", Category: ErrCategoryClient, Message: "ig_user_id required in metadata"}
	}

	// Step 1: Create media container
	containerID, err := c.createContainer(ctx, req)
	if err != nil {
		return nil, err
	}

	// Step 2: Wait for container to be ready (poll)
	if err := c.waitReady(ctx, containerID); err != nil {
		return nil, err
	}

	// Step 3: Publish container
	return c.publishContainer(ctx, containerID)
}

func (c *instagramClient) createContainer(ctx context.Context, req PublishRequest) (string, error) {
	baseURL := fmt.Sprintf("https://graph.facebook.com/%s/%s/media", GraphVersion, c.igID)
	body := map[string]any{
		"caption":      req.Content,
		"access_token": c.token,
	}

	if len(req.Media) > 0 {
		m := req.Media[0]
		switch m.MediaType {
		case "image":
			body["image_url"] = m.URL
		case "video":
			body["video_url"] = m.URL
			body["media_type"] = "REELS"
		}
	}

	// Carousel if multiple images
	if len(req.Media) > 1 {
		var children []string
		for _, m := range req.Media {
			child, err := c.createChildContainer(ctx, m)
			if err != nil {
				return "", err
			}
			children = append(children, child)
		}
		body["media_type"] = "CAROUSEL"
		body["children"] = children
		delete(body, "image_url")
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := doJSON(ctx, "POST", baseURL, body, nil, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (c *instagramClient) createChildContainer(ctx context.Context, m MediaItem) (string, error) {
	baseURL := fmt.Sprintf("https://graph.facebook.com/%s/%s/media", GraphVersion, c.igID)
	body := map[string]any{
		"is_carousel_item": true,
		"access_token":     c.token,
	}
	if m.MediaType == "image" {
		body["image_url"] = m.URL
	} else {
		body["video_url"] = m.URL
		body["media_type"] = "VIDEO"
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := doJSON(ctx, "POST", baseURL, body, nil, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (c *instagramClient) waitReady(ctx context.Context, containerID string) error {
	for i := 0; i < 30; i++ {
		apiURL := fmt.Sprintf("https://graph.facebook.com/%s/%s?fields=status_code&access_token=%s", GraphVersion, containerID, c.token)
		var resp struct {
			StatusCode string `json:"status_code"`
		}
		if err := doJSON(ctx, "GET", apiURL, nil, nil, &resp); err != nil {
			return err
		}
		if resp.StatusCode == "FINISHED" {
			return nil
		}
		if resp.StatusCode == "ERROR" {
			return &PlatformError{Platform: "instagram", Category: ErrCategoryPlatform, Message: "container failed"}
		}
		time.Sleep(2 * time.Second)
	}
	return &PlatformError{Platform: "instagram", Category: ErrCategoryPlatform, Message: "container processing timeout"}
}

func (c *instagramClient) publishContainer(ctx context.Context, containerID string) (*PublishResult, error) {
	baseURL := fmt.Sprintf("https://graph.facebook.com/%s/%s/media_publish", GraphVersion, c.igID)
	body := map[string]any{
		"creation_id":  containerID,
		"access_token": c.token,
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := doJSON(ctx, "POST", baseURL, body, nil, &resp); err != nil {
		return nil, err
	}
	return &PublishResult{
		PlatformPostID: resp.ID,
		PlatformURL:    fmt.Sprintf("https://instagram.com/p/%s", resp.ID),
	}, nil
}

func (c *instagramClient) RefreshToken(ctx context.Context, refreshToken string) (*TokenResult, error) {
	apiURL := fmt.Sprintf("https://graph.facebook.com/%s/oauth/access_token?grant_type=ig_refresh_token&access_token=%s", GraphVersion, c.token)
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

func (c *instagramClient) GetProfile(ctx context.Context) (*ProfileResult, error) {
	apiURL := fmt.Sprintf("https://graph.facebook.com/%s/%s?fields=id,username,name,profile_picture_url&access_token=%s", GraphVersion, c.igID, c.token)
	var resp struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		User    string `json:"username"`
		Picture string `json:"profile_picture_url"`
	}
	if err := doJSON(ctx, "GET", apiURL, nil, nil, &resp); err != nil {
		return nil, err
	}
	return &ProfileResult{ID: resp.ID, Username: resp.User, Name: resp.Name, Avatar: resp.Picture}, nil
}
