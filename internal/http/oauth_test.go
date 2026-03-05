package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOAuthHandlerStatusNoToken(t *testing.T) {
	h := NewOAuthHandler("", "")
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/auth/openai/status", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)

	if result["authenticated"] != false {
		t.Errorf("authenticated = %v, want false", result["authenticated"])
	}
}

func TestOAuthHandlerAuth(t *testing.T) {
	h := NewOAuthHandler("secret-token", "")
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Without token - should be unauthorized
	req := httptest.NewRequest("GET", "/v1/auth/openai/status", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status code without token = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	// With correct token - should work
	req2 := httptest.NewRequest("GET", "/v1/auth/openai/status", nil)
	req2.Header.Set("Authorization", "Bearer secret-token")
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("status code with token = %d, want %d", w2.Code, http.StatusOK)
	}
}

func TestOAuthHandlerLogoutNoToken(t *testing.T) {
	h := NewOAuthHandler("", "")
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/v1/auth/openai/logout", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	var result map[string]string
	json.NewDecoder(w.Body).Decode(&result)

	if result["status"] != "no token found" {
		t.Errorf("status = %q, want 'no token found'", result["status"])
	}
}

func TestOAuthHandlerStartReturnsAuthURL(t *testing.T) {
	h := NewOAuthHandler("", "")
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/v1/auth/openai/start", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)

	// Should return either auth_url or already_authenticated status
	authURL, hasURL := result["auth_url"]
	status, hasStatus := result["status"]

	if !hasURL && !hasStatus {
		t.Fatal("response has neither auth_url nor status")
	}

	if hasURL {
		urlStr, ok := authURL.(string)
		if !ok || urlStr == "" {
			t.Errorf("auth_url = %v, expected non-empty string", authURL)
		}
		if !strings.HasPrefix(urlStr, "https://auth.openai.com") {
			t.Errorf("auth_url doesn't start with https://auth.openai.com: %s", urlStr)
		}
	}

	if hasStatus {
		if status != "already_authenticated" {
			t.Errorf("status = %v, expected already_authenticated", status)
		}
	}
}

func TestOAuthHandlerRouteRegistration(t *testing.T) {
	h := NewOAuthHandler("", "")
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// GET /v1/auth/openai/status should work
	req := httptest.NewRequest("GET", "/v1/auth/openai/status", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code == http.StatusNotFound {
		t.Error("GET /v1/auth/openai/status returned 404")
	}

	// POST /v1/auth/openai/logout should work
	req2 := httptest.NewRequest("POST", "/v1/auth/openai/logout", nil)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)
	if w2.Code == http.StatusNotFound {
		t.Error("POST /v1/auth/openai/logout returned 404")
	}

	// POST /v1/auth/openai/start should work
	req3 := httptest.NewRequest("POST", "/v1/auth/openai/start", nil)
	w3 := httptest.NewRecorder()
	mux.ServeHTTP(w3, req3)
	if w3.Code == http.StatusNotFound {
		t.Error("POST /v1/auth/openai/start returned 404")
	}
}
