package social

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type youtubeClient struct {
	token string
}

func newYouTubeClient(token string, _ json.RawMessage) *youtubeClient {
	return &youtubeClient{token: token}
}

func (c *youtubeClient) Platform() string { return "youtube" }

func (c *youtubeClient) Publish(ctx context.Context, req PublishRequest) (*PublishResult, error) {
	if len(req.Media) == 0 {
		return nil, &PlatformError{Platform: "youtube", Category: ErrCategoryClient, Message: "video media required"}
	}

	// YouTube requires video upload. Use resumable upload API.
	title := req.Content
	if t, ok := req.Metadata["title"].(string); ok && t != "" {
		title = t
	}
	if len([]rune(title)) > 100 {
		title = string([]rune(title)[:100])
	}

	privacy := "public"
	if p, ok := req.Metadata["privacy"].(string); ok && p != "" {
		privacy = p
	}

	snippet := map[string]any{
		"title":       title,
		"description": req.Content,
		"categoryId":  "22", // People & Blogs
	}
	body := map[string]any{
		"snippet": snippet,
		"status":  map[string]string{"privacyStatus": privacy},
	}

	var resp struct {
		ID string `json:"id"`
	}
	apiURL := "https://www.googleapis.com/youtube/v3/videos?part=snippet,status&uploadType=resumable"
	if err := doJSON(ctx, "POST", apiURL, body, bearerHeader(c.token), &resp); err != nil {
		return nil, err
	}
	return &PublishResult{
		PlatformPostID: resp.ID,
		PlatformURL:    fmt.Sprintf("https://youtube.com/watch?v=%s", resp.ID),
	}, nil
}

func (c *youtubeClient) RefreshToken(ctx context.Context, refreshToken string) (*TokenResult, error) {
	var resp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	apiURL := fmt.Sprintf("https://oauth2.googleapis.com/token?grant_type=refresh_token&refresh_token=%s", refreshToken)
	if err := doJSON(ctx, "POST", apiURL, nil, nil, &resp); err != nil {
		return nil, err
	}
	return &TokenResult{
		AccessToken: resp.AccessToken,
		ExpiresAt:   time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second),
	}, nil
}

func (c *youtubeClient) GetProfile(ctx context.Context) (*ProfileResult, error) {
	apiURL := "https://www.googleapis.com/youtube/v3/channels?part=snippet&mine=true"
	var resp struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title      string `json:"title"`
				CustomURL  string `json:"customUrl"`
				Thumbnails struct {
					Default struct {
						URL string `json:"url"`
					} `json:"default"`
				} `json:"thumbnails"`
			} `json:"snippet"`
		} `json:"items"`
	}
	if err := doJSON(ctx, "GET", apiURL, nil, bearerHeader(c.token), &resp); err != nil {
		return nil, err
	}
	if len(resp.Items) == 0 {
		return nil, &PlatformError{Platform: "youtube", Category: ErrCategoryPlatform, Message: "no channel found"}
	}
	ch := resp.Items[0]
	return &ProfileResult{
		ID: ch.ID, Username: ch.Snippet.CustomURL,
		Name: ch.Snippet.Title, Avatar: ch.Snippet.Thumbnails.Default.URL,
	}, nil
}
