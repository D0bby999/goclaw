package session

import (
	"encoding/json"
	"fmt"

	"github.com/nextlevelbuilder/goclaw/internal/crypto"
)

// EncryptCookies serializes a cookie map to JSON and encrypts it.
func EncryptCookies(cookies map[string]string, key string) (string, error) {
	data, err := json.Marshal(cookies)
	if err != nil {
		return "", fmt.Errorf("marshal cookies: %w", err)
	}
	encrypted, err := crypto.Encrypt(string(data), key)
	if err != nil {
		return "", fmt.Errorf("encrypt cookies: %w", err)
	}
	return encrypted, nil
}

// DecryptCookies decrypts a ciphertext and deserializes it back to a cookie map.
func DecryptCookies(ciphertext, key string) (map[string]string, error) {
	plaintext, err := crypto.Decrypt(ciphertext, key)
	if err != nil {
		return nil, fmt.Errorf("decrypt cookies: %w", err)
	}
	var cookies map[string]string
	if err := json.Unmarshal([]byte(plaintext), &cookies); err != nil {
		return nil, fmt.Errorf("unmarshal cookies: %w", err)
	}
	return cookies, nil
}
