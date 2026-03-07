package social

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

type facebookClient struct {
	token    string
	pageID   string
	pageTok  string
}

func newFacebookClient(token string, metadata json.RawMessage) *facebookClient {
	c := &facebookClient{token: token}
	if metadata != nil {
		var m map[string]string
		if json.Unmarshal(metadata, &m) == nil {
			c.pageID = m["page_id"]
			c.pageTok = m["page_token"]
		}
	}
	return c
}

func (c *facebookClient) Platform() string { return "facebook" }

func (c *facebookClient) Publish(ctx context.Context, req PublishRequest) (*PublishResult, error) {
	tok := c.pageTok
	if tok == "" {
		tok = c.token
	}
	target := c.pageID
	if target == "" {
		target = "me"
	}

	// Text post or photo post
	if len(req.Media) > 0 && req.Media[0].MediaType == "image" {
		return c.publishPhoto(ctx, target, tok, req)
	}
	return c.publishText(ctx, target, tok, req)
}

func (c *facebookClient) publishText(ctx context.Context, target, tok string, req PublishRequest) (*PublishResult, error) {
	apiURL := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/feed", target)
	params := url.Values{"message": {req.Content}, "access_token": {tok}}

	var resp struct {
		ID string `json:"id"`
	}
	if err := doJSON(ctx, "POST", apiURL+"?"+params.Encode(), nil, nil, &resp); err != nil {
		return nil, err
	}
	return &PublishResult{
		PlatformPostID: resp.ID,
		PlatformURL:    fmt.Sprintf("https://facebook.com/%s", resp.ID),
	}, nil
}

func (c *facebookClient) publishPhoto(ctx context.Context, target, tok string, req PublishRequest) (*PublishResult, error) {
	apiURL := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/photos", target)
	params := url.Values{
		"url":          {req.Media[0].URL},
		"caption":      {req.Content},
		"access_token": {tok},
	}

	var resp struct {
		ID     string `json:"id"`
		PostID string `json:"post_id"`
	}
	if err := doJSON(ctx, "POST", apiURL+"?"+params.Encode(), nil, nil, &resp); err != nil {
		return nil, err
	}
	postID := resp.PostID
	if postID == "" {
		postID = resp.ID
	}
	return &PublishResult{
		PlatformPostID: postID,
		PlatformURL:    fmt.Sprintf("https://facebook.com/%s", postID),
	}, nil
}

func (c *facebookClient) RefreshToken(ctx context.Context, refreshToken string) (*TokenResult, error) {
	// Facebook long-lived tokens don't use standard refresh; exchange for new long-lived token
	return nil, &PlatformError{Platform: "facebook", Category: ErrCategoryPlatform, Message: "use token exchange instead"}
}

func (c *facebookClient) GetProfile(ctx context.Context) (*ProfileResult, error) {
	tok := c.pageTok
	if tok == "" {
		tok = c.token
	}
	apiURL := fmt.Sprintf("https://graph.facebook.com/v19.0/me?fields=id,name,picture&access_token=%s", tok)
	var resp struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Picture struct {
			Data struct {
				URL string `json:"url"`
			} `json:"data"`
		} `json:"picture"`
	}
	if err := doJSON(ctx, "GET", apiURL, nil, nil, &resp); err != nil {
		return nil, err
	}
	return &ProfileResult{ID: resp.ID, Name: resp.Name, Avatar: resp.Picture.Data.URL}, nil
}
