package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"
)

var randReader io.Reader = rand.Reader

// GenerateAuthorization creates an HMAC-SHA256 Authorization header value.
// Format: HMAC-SHA256 apiKey=<key>, date=<RFC3339>, salt=<hex>, signature=<hex>
func GenerateAuthorization(apiKey, apiSecret string) (string, error) {
	salt, err := randomHex(20)
	if err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}
	date := time.Now().Format(time.RFC3339)
	signature := sign(date+salt, apiSecret)
	return fmt.Sprintf("HMAC-SHA256 apiKey=%s, date=%s, salt=%s, signature=%s", apiKey, date, salt, signature), nil
}

// GenerateAuthorizationWithParams is a testable variant that accepts date and salt.
func GenerateAuthorizationWithParams(apiKey, apiSecret, date, salt string) string {
	signature := sign(date+salt, apiSecret)
	return fmt.Sprintf("HMAC-SHA256 apiKey=%s, date=%s, salt=%s, signature=%s", apiKey, date, salt, signature)
}

func sign(data, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(randReader, b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
