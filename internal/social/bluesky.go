package social

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type blueskyClient struct {
	token  string
	handle string
	did    string
}

func newBlueskyClient(token string, metadata json.RawMessage) *blueskyClient {
	c := &blueskyClient{token: token}
	if metadata != nil {
		var m map[string]string
		if json.Unmarshal(metadata, &m) == nil {
			c.handle = m["handle"]
			c.did = m["did"]
		}
	}
	return c
}

func (c *blueskyClient) Platform() string { return "bluesky" }

func (c *blueskyClient) Publish(ctx context.Context, req PublishRequest) (*PublishResult, error) {
	repo := c.did
	if repo == "" {
		repo = c.handle
	}
	if repo == "" {
		return nil, &PlatformError{Platform: "bluesky", Category: ErrCategoryClient, Message: "did or handle required"}
	}

	record := map[string]any{
		"$type":     "app.bsky.feed.post",
		"text":      req.Content,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	// Embed images if present
	if len(req.Media) > 0 {
		var images []map[string]any
		for _, m := range req.Media {
			if m.MediaType == "image" {
				blobRef, err := c.uploadBlob(ctx, m)
				if err != nil {
					return nil, err
				}
				images = append(images, map[string]any{
					"alt":   "",
					"image": blobRef,
				})
			}
		}
		if len(images) > 0 {
			record["embed"] = map[string]any{
				"$type":  "app.bsky.embed.images",
				"images": images,
			}
		}
	}

	// Reply support
	if req.ReplyTo != "" {
		record["reply"] = map[string]any{
			"root":   map[string]any{"uri": req.ReplyTo, "cid": ""},
			"parent": map[string]any{"uri": req.ReplyTo, "cid": ""},
		}
	}

	body := map[string]any{
		"repo":       repo,
		"collection": "app.bsky.feed.post",
		"record":     record,
	}

	var resp struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	}
	if err := doJSON(ctx, "POST", "https://bsky.social/xrpc/com.atproto.repo.createRecord",
		body, bearerHeader(c.token), &resp); err != nil {
		return nil, err
	}
	return &PublishResult{
		PlatformPostID: resp.URI,
		PlatformURL:    fmt.Sprintf("https://bsky.app/profile/%s/post/%s", c.handle, extractRKey(resp.URI)),
	}, nil
}

func (c *blueskyClient) uploadBlob(ctx context.Context, m MediaItem) (map[string]any, error) {
	// Bluesky blob upload: POST binary data to uploadBlob endpoint
	// For URL-based media, we'd need to download first then upload
	// Simplified: return a placeholder blob reference
	return map[string]any{
		"$type": "blob",
		"ref":   map[string]string{"$link": m.URL},
		"mimeType": m.MimeType,
		"size":     0,
	}, nil
}

func (c *blueskyClient) RefreshToken(ctx context.Context, refreshToken string) (*TokenResult, error) {
	var resp struct {
		AccessJwt  string `json:"accessJwt"`
		RefreshJwt string `json:"refreshJwt"`
	}
	headers := map[string]string{"Authorization": "Bearer " + refreshToken}
	if err := doJSON(ctx, "POST", "https://bsky.social/xrpc/com.atproto.server.refreshSession",
		nil, headers, &resp); err != nil {
		return nil, err
	}
	return &TokenResult{
		AccessToken:  resp.AccessJwt,
		RefreshToken: resp.RefreshJwt,
		ExpiresAt:    time.Now().Add(2 * time.Hour), // AT Protocol access tokens ~2h
	}, nil
}

func (c *blueskyClient) GetProfile(ctx context.Context) (*ProfileResult, error) {
	actor := c.handle
	if actor == "" {
		actor = c.did
	}
	apiURL := fmt.Sprintf("https://bsky.social/xrpc/app.bsky.actor.getProfile?actor=%s", actor)
	var resp struct {
		DID         string `json:"did"`
		Handle      string `json:"handle"`
		DisplayName string `json:"displayName"`
		Avatar      string `json:"avatar"`
	}
	if err := doJSON(ctx, "GET", apiURL, nil, bearerHeader(c.token), &resp); err != nil {
		return nil, err
	}
	return &ProfileResult{ID: resp.DID, Username: resp.Handle, Name: resp.DisplayName, Avatar: resp.Avatar}, nil
}

// extractRKey extracts the record key from an AT Protocol URI.
func extractRKey(uri string) string {
	// at://did:plc:xxx/app.bsky.feed.post/rkey
	parts := splitLast(uri, "/")
	return parts
}

func splitLast(s, sep string) string {
	idx := len(s) - 1
	for idx >= 0 {
		if string(s[idx]) == sep {
			return s[idx+1:]
		}
		idx--
	}
	return s
}
