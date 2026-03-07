package social

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// doJSON performs an HTTP request with JSON body and decodes the JSON response.
func doJSON(ctx context.Context, method, url string, body any, headers map[string]string, out any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return &PlatformError{Category: ErrCategoryNetwork, Message: err.Error()}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return classifyHTTPError(resp.StatusCode, respBody, "")
	}
	if out != nil && len(respBody) > 0 {
		return json.Unmarshal(respBody, out)
	}
	return nil
}

// classifyHTTPError maps HTTP status codes to error categories.
func classifyHTTPError(status int, body []byte, platform string) *PlatformError {
	cat := ErrCategoryPlatform
	switch {
	case status == 401 || status == 403:
		cat = ErrCategoryAuth
	case status == 429:
		cat = ErrCategoryRateLimit
	case status >= 400 && status < 500:
		cat = ErrCategoryClient
	}
	msg := string(body)
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return &PlatformError{Platform: platform, Category: cat, Code: status, Message: msg}
}

// bearerHeader returns an Authorization: Bearer header map.
func bearerHeader(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}
