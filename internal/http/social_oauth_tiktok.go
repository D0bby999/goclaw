package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/social"
)

const (
	tiktokAuthURL  = "https://www.tiktok.com/v2/auth/authorize/"
	tiktokTokenURL = "https://open.tiktokapis.com/v2/oauth/token/"
)

// buildTikTokAuthURL builds the TikTok OAuth 2.0 authorization URL.
// NOTE: TikTok uses client_key (not client_id) in the auth URL.
// Scopes are comma-separated (not space-separated like other platforms).
func buildTikTokAuthURL(clientKey, redirectURI, state string) string {
	params := url.Values{
		"client_key":    {clientKey},
		"redirect_uri":  {redirectURI},
		"scope":         {"user.info.basic,video.publish,video.upload"},
		"state":         {state},
		"response_type": {"code"},
	}
	return tiktokAuthURL + "?" + params.Encode()
}

// exchangeTikTokCode exchanges the authorization code for tokens.
// Returns tokenResponse with OpenID set from TikTok's open_id field.
func exchangeTikTokCode(ctx context.Context, cfg *social.OAuthConfig, code, redirectURI string) (*tokenResponse, error) {
	body := url.Values{
		"client_key":    {cfg.ClientID},
		"client_secret": {cfg.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {redirectURI},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tiktokTokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxOAuthResponseBody))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tiktok token exchange failed (%d): %s", resp.StatusCode, string(data))
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		OpenID       string `json:"open_id"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	tok := &tokenResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresIn,
		OpenID:       result.OpenID,
	}
	if result.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
		tok.ExpiresAt = &t
	}
	return tok, nil
}

// fetchTikTokProfile fetches the authenticated user's TikTok profile.
// NOTE: TikTok's user info endpoint is a POST with a JSON body (not a GET).
func fetchTikTokProfile(ctx context.Context, token string) (*social.ProfileResult, error) {
	bodyData, _ := json.Marshal(map[string]any{
		"fields": []string{"open_id", "display_name", "avatar_url"},
	})

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.tiktokapis.com/v2/user/info/", bytes.NewReader(bodyData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxOAuthResponseBody))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tiktok profile fetch failed (%d): %s", resp.StatusCode, string(data))
	}

	var result struct {
		Data struct {
			User struct {
				OpenID      string `json:"open_id"`
				DisplayName string `json:"display_name"`
				AvatarURL   string `json:"avatar_url"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	u := result.Data.User
	return &social.ProfileResult{
		ID:     u.OpenID,
		Name:   u.DisplayName,
		Avatar: u.AvatarURL,
	}, nil
}
