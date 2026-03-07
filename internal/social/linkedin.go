package social

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type linkedinClient struct {
	token    string
	personID string
}

func newLinkedInClient(token string, metadata json.RawMessage) *linkedinClient {
	c := &linkedinClient{token: token}
	if metadata != nil {
		var m map[string]string
		if json.Unmarshal(metadata, &m) == nil {
			c.personID = m["person_id"]
		}
	}
	return c
}

func (c *linkedinClient) Platform() string { return "linkedin" }

func (c *linkedinClient) Publish(ctx context.Context, req PublishRequest) (*PublishResult, error) {
	author := fmt.Sprintf("urn:li:person:%s", c.personID)
	if c.personID == "" {
		// Fetch profile to get person URN
		profile, err := c.GetProfile(ctx)
		if err != nil {
			return nil, err
		}
		author = fmt.Sprintf("urn:li:person:%s", profile.ID)
	}

	body := map[string]any{
		"author":     author,
		"commentary": req.Content,
		"visibility": "PUBLIC",
		"distribution": map[string]any{
			"feedDistribution":               "MAIN_FEED",
			"targetEntities":                 []any{},
			"thirdPartyDistributionChannels": []any{},
		},
		"lifecycleState": "PUBLISHED",
	}

	// Media attachment
	if len(req.Media) > 0 {
		body["content"] = map[string]any{
			"media": map[string]any{
				"title": req.Content,
				"id":    req.Media[0].URL, // pre-registered media URN
			},
		}
	}

	headers := map[string]string{
		"Authorization":    "Bearer " + c.token,
		"LinkedIn-Version": "202401",
		"X-Restli-Protocol-Version": "2.0.0",
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := doJSON(ctx, "POST", "https://api.linkedin.com/rest/posts", body, headers, &resp); err != nil {
		return nil, err
	}
	return &PublishResult{
		PlatformPostID: resp.ID,
		PlatformURL:    fmt.Sprintf("https://linkedin.com/feed/update/%s", resp.ID),
	}, nil
}

func (c *linkedinClient) RefreshToken(ctx context.Context, refreshToken string) (*TokenResult, error) {
	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	apiURL := fmt.Sprintf("https://www.linkedin.com/oauth/v2/accessToken?grant_type=refresh_token&refresh_token=%s", refreshToken)
	if err := doJSON(ctx, "POST", apiURL, nil, nil, &resp); err != nil {
		return nil, err
	}
	return &TokenResult{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second),
	}, nil
}

func (c *linkedinClient) GetProfile(ctx context.Context) (*ProfileResult, error) {
	var resp struct {
		Sub     string `json:"sub"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := doJSON(ctx, "GET", "https://api.linkedin.com/v2/userinfo",
		nil, bearerHeader(c.token), &resp); err != nil {
		return nil, err
	}
	return &ProfileResult{ID: resp.Sub, Name: resp.Name, Avatar: resp.Picture}, nil
}
