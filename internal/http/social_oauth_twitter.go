package http

import (
	"context"
	"encoding/base64"
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
	twitterAuthURL  = "https://twitter.com/i/oauth2/authorize"
	twitterTokenURL = "https://api.x.com/2/oauth2/token"
)

// buildTwitterAuthURL builds the Twitter OAuth 2.0 PKCE authorization URL.
// Returns the auth URL, state metadata (containing code_verifier), and any error.
func buildTwitterAuthURL(clientID, redirectURI, state string) (authURL string, stateMeta json.RawMessage, err error) {
	verifier, err := generateCodeVerifier()
	if err != nil {
		return "", nil, fmt.Errorf("generate code verifier: %w", err)
	}
	challenge := s256Challenge(verifier)

	scopes := "tweet.read tweet.write users.read offline.access"
	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {scopes},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	authURL = twitterAuthURL + "?" + params.Encode()

	// Persist code_verifier in state metadata for callback retrieval.
	meta, err := json.Marshal(map[string]string{"code_verifier": verifier})
	if err != nil {
		return "", nil, fmt.Errorf("marshal state meta: %w", err)
	}
	return authURL, json.RawMessage(meta), nil
}

// exchangeTwitterCode exchanges the authorization code for tokens using PKCE.
func exchangeTwitterCode(ctx context.Context, cfg *social.OAuthConfig, code, redirectURI, codeVerifier string) (*tokenResponse, error) {
	body := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {codeVerifier},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", twitterTokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Twitter requires Basic auth: base64(client_id:client_secret)
	creds := base64.StdEncoding.EncodeToString([]byte(cfg.ClientID + ":" + cfg.ClientSecret))
	req.Header.Set("Authorization", "Basic "+creds)

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
		return nil, fmt.Errorf("twitter token exchange failed (%d): %s", resp.StatusCode, string(data))
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
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

// fetchTwitterProfile fetches the authenticated user's Twitter profile.
func fetchTwitterProfile(ctx context.Context, token string) (*social.ProfileResult, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://api.x.com/2/users/me?user.fields=profile_image_url,username", nil)
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
		return nil, fmt.Errorf("twitter profile fetch failed (%d): %s", resp.StatusCode, string(data))
	}

	var result struct {
		Data struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Username string `json:"username"`
			Avatar   string `json:"profile_image_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &social.ProfileResult{
		ID:       result.Data.ID,
		Username: result.Data.Username,
		Name:     result.Data.Name,
		Avatar:   result.Data.Avatar,
	}, nil
}
