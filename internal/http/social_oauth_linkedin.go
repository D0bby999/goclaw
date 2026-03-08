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
	linkedinAuthURL  = "https://www.linkedin.com/oauth/v2/authorization"
	linkedinTokenURL = "https://www.linkedin.com/oauth/v2/accessToken"
)

// buildLinkedInAuthURL builds the LinkedIn OAuth 2.0 authorization URL.
func buildLinkedInAuthURL(clientID, redirectURI, state string) string {
	params := url.Values{
		"response_type": {"code"},
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"scope":         {"openid profile w_member_social"},
		"state":         {state},
	}
	return linkedinAuthURL + "?" + params.Encode()
}

// exchangeLinkedInCode exchanges the authorization code for tokens.
func exchangeLinkedInCode(ctx context.Context, cfg *social.OAuthConfig, code, redirectURI string) (*tokenResponse, error) {
	body := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {cfg.ClientID},
		"client_secret": {cfg.ClientSecret},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", linkedinTokenURL, strings.NewReader(body.Encode()))
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
		return nil, fmt.Errorf("linkedin token exchange failed (%d): %s", resp.StatusCode, string(data))
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

// fetchLinkedInProfile fetches the authenticated user's LinkedIn profile via OpenID Connect.
func fetchLinkedInProfile(ctx context.Context, token string) (*social.ProfileResult, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.linkedin.com/v2/userinfo", nil)
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
		return nil, fmt.Errorf("linkedin profile fetch failed (%d): %s", resp.StatusCode, string(data))
	}

	var result struct {
		Sub     string `json:"sub"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &social.ProfileResult{
		ID:     result.Sub,
		Name:   result.Name,
		Avatar: result.Picture,
	}, nil
}
