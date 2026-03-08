package http

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// generateCodeVerifier generates a PKCE code verifier (43+ random bytes → base64url no padding).
func generateCodeVerifier() (string, error) {
	b := make([]byte, 43)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// s256Challenge derives the PKCE code_challenge from a verifier using S256 method.
func s256Challenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
