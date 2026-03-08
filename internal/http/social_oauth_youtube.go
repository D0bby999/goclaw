package http

import (
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
	youtubeAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	youtubeTokenURL = "https://oauth2.googleapis.com/token"
)

// buildYouTubeAuthURL builds the Google OAuth 2.0 authorization URL for YouTube.
// Requests offline access + forces consent screen to always get a refresh token.
func buildYouTubeAuthURL(clientID, redirectURI, state string) string {
	scopes := "https://www.googleapis.com/auth/youtube.upload https://www.googleapis.com/auth/youtube.readonly"
	params := url.Values{
		"response_type": {"code"},
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"scope":         {scopes},
		"state":         {state},
		"access_type":   {"offline"},
		"prompt":        {"consent"},
	}
	return youtubeAuthURL + "?" + params.Encode()
}

// exchangeYouTubeCode exchanges the authorization code for tokens.
func exchangeYouTubeCode(ctx context.Context, cfg *social.OAuthConfig, code, redirectURI string) (*tokenResponse, error) {
	body := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {cfg.ClientID},
		"client_secret": {cfg.ClientSecret},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", youtubeTokenURL, strings.NewReader(body.Encode()))
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
		return nil, fmt.Errorf("youtube token exchange failed (%d): %s", resp.StatusCode, string(data))
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	tok := &tokenResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresIn,
	}
	if result.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
		tok.ExpiresAt = &t
	}
	return tok, nil
}

// fetchYouTubeProfile fetches the authenticated user's YouTube channel info.
func fetchYouTubeProfile(ctx context.Context, token string) (*social.ProfileResult, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://www.googleapis.com/youtube/v3/channels?part=snippet&mine=true", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

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
		return nil, fmt.Errorf("youtube profile fetch failed (%d): %s", resp.StatusCode, string(data))
	}

	var result struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title     string `json:"title"`
				CustomURL string `json:"customUrl"`
				Thumbnails struct {
					Default struct {
						URL string `json:"url"`
					} `json:"default"`
				} `json:"thumbnails"`
			} `json:"snippet"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if len(result.Items) == 0 {
		return nil, fmt.Errorf("no YouTube channel found")
	}

	ch := result.Items[0]
	profile := &social.ProfileResult{
		ID:       ch.ID,
		Username: ch.Snippet.CustomURL,
		Name:     ch.Snippet.Title,
		Avatar:   ch.Snippet.Thumbnails.Default.URL,
	}
	return profile, nil
}
