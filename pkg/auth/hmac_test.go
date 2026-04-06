package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
)

func TestGenerateAuthorizationWithParams_KnownOutput(t *testing.T) {
	apiKey := "NCS1234567890ABC"
	apiSecret := "01234567890123456789012345678901"
	date := "2026-01-01T00:00:00Z"
	salt := "abcdef0123456789abcdef0123456789abcdef01"

	result := GenerateAuthorizationWithParams(apiKey, apiSecret, date, salt)

	// Compute expected signature
	h := hmac.New(sha256.New, []byte(apiSecret))
	h.Write([]byte(date + salt))
	expectedSig := hex.EncodeToString(h.Sum(nil))

	expected := "HMAC-SHA256 apiKey=" + apiKey + ", date=" + date + ", salt=" + salt + ", signature=" + expectedSig
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestGenerateAuthorization_Format(t *testing.T) {
	apiKey := "NCS1234567890ABC"
	apiSecret := "01234567890123456789012345678901"

	result, err := GenerateAuthorization(apiKey, apiSecret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pattern := `^HMAC-SHA256 apiKey=.+, date=.+, salt=[0-9a-f]{40}, signature=[0-9a-f]{64}$`
	matched, err := regexp.MatchString(pattern, result)
	if err != nil {
		t.Fatalf("regex error: %v", err)
	}
	if !matched {
		t.Errorf("format mismatch: %q", result)
	}
}

func TestGenerateAuthorization_Randomness(t *testing.T) {
	apiKey := "NCS1234567890ABC"
	apiSecret := "01234567890123456789012345678901"

	r1, err := GenerateAuthorization(apiKey, apiSecret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r2, err := GenerateAuthorization(apiKey, apiSecret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// salt differs -> signature differs -> full string differs
	if r1 == r2 {
		t.Error("two consecutive calls produced identical output; salt should be random")
	}
}

func TestGenerateAuthorizationWithParams_EmptyInputs(t *testing.T) {
	tests := []struct {
		name      string
		apiKey    string
		apiSecret string
	}{
		{"empty apiKey", "", "01234567890123456789012345678901"},
		{"empty apiSecret", "NCS1234567890ABC", ""},
		{"both empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateAuthorizationWithParams(tt.apiKey, tt.apiSecret, "2026-01-01T00:00:00Z", "aabb")
			if !strings.HasPrefix(result, "HMAC-SHA256 ") {
				t.Errorf("unexpected prefix: %q", result)
			}
			// Should not panic; signature is still generated (HMAC with empty key is valid)
		})
	}
}

func TestSign_Deterministic(t *testing.T) {
	s1 := sign("hello", "secret")
	s2 := sign("hello", "secret")
	if s1 != s2 {
		t.Errorf("sign not deterministic: %q vs %q", s1, s2)
	}
}

func TestSign_DifferentInputsDifferentOutput(t *testing.T) {
	s1 := sign("hello", "secret")
	s2 := sign("world", "secret")
	if s1 == s2 {
		t.Error("different inputs produced same signature")
	}
}

func TestRandomHex_Length(t *testing.T) {
	h, err := randomHex(20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(h) != 40 {
		t.Errorf("expected 40 hex chars, got %d", len(h))
	}
}

type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, fmt.Errorf("entropy failure") }

func TestRandomHex_ReaderFailure(t *testing.T) {
	orig := randReader
	randReader = failReader{}
	t.Cleanup(func() { randReader = orig })

	_, err := GenerateAuthorization("key", "secret")
	if err == nil {
		t.Fatal("expected error when randReader fails, got nil")
	}
	if !strings.Contains(err.Error(), "generating salt") {
		t.Errorf("error should contain 'generating salt', got: %v", err)
	}
}

func TestGenerateAuthorization_SaltError(t *testing.T) {
	orig := randReader
	randReader = failReader{}
	t.Cleanup(func() { randReader = orig })

	_, err := GenerateAuthorization("NCS1234567890ABC", "01234567890123456789012345678901")
	if err == nil {
		t.Fatal("expected error when salt generation fails, got nil")
	}
}

func TestRandomHex_Zero(t *testing.T) {
	h, err := randomHex(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h != "" {
		t.Errorf("expected empty string for randomHex(0), got %q", h)
	}
}

func TestRandomHex_Large(t *testing.T) {
	h, err := randomHex(1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(h) != 2000 {
		t.Errorf("expected 2000 hex chars, got %d", len(h))
	}
	// Verify all characters are valid hex
	for i, c := range h {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("non-hex char %q at index %d", string(c), i)
			break
		}
	}
}

func TestGenerateAuthorizationWithParams_SpecialCharsInSecret(t *testing.T) {
	apiKey := "NCS1234567890ABC"
	// Non-ASCII secret with unicode characters
	apiSecret := "비밀키🔑café\x00\xff"
	date := "2026-01-01T00:00:00Z"
	salt := "aabbccdd"

	result := GenerateAuthorizationWithParams(apiKey, apiSecret, date, salt)

	// Compute expected signature independently
	h := hmac.New(sha256.New, []byte(apiSecret))
	h.Write([]byte(date + salt))
	expectedSig := hex.EncodeToString(h.Sum(nil))

	expected := fmt.Sprintf("HMAC-SHA256 apiKey=%s, date=%s, salt=%s, signature=%s", apiKey, date, salt, expectedSig)
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestGenerateAuthorizationWithParams_VeryLongInputs(t *testing.T) {
	apiKey := strings.Repeat("K", 10000)
	apiSecret := strings.Repeat("S", 10000)
	date := "2026-01-01T00:00:00Z"
	salt := "aabbccdd"

	result := GenerateAuthorizationWithParams(apiKey, apiSecret, date, salt)

	// Compute expected signature independently
	h := hmac.New(sha256.New, []byte(apiSecret))
	h.Write([]byte(date + salt))
	expectedSig := hex.EncodeToString(h.Sum(nil))

	expected := fmt.Sprintf("HMAC-SHA256 apiKey=%s, date=%s, salt=%s, signature=%s", apiKey, date, salt, expectedSig)
	if result != expected {
		t.Errorf("result does not match expected output for very long inputs")
	}
	if !strings.Contains(result, apiKey) {
		t.Error("result should contain the full apiKey")
	}
}

func FuzzRandomHex(f *testing.F) {
	f.Add(uint8(0))
	f.Add(uint8(1))
	f.Add(uint8(20))
	f.Add(uint8(255))

	f.Fuzz(func(t *testing.T, n uint8) {
		size := int(n) % 256
		h, err := randomHex(size)
		if err != nil {
			t.Fatalf("randomHex(%d) returned error: %v", size, err)
		}
		if len(h) != size*2 {
			t.Errorf("randomHex(%d) returned %d chars, want %d", size, len(h), size*2)
		}
	})
}

func TestGenerateAuthorization_Concurrent(t *testing.T) {
	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	errs := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			result, err := GenerateAuthorization("NCS1234567890ABC", "01234567890123456789012345678901")
			if err != nil {
				errs <- fmt.Errorf("unexpected error: %w", err)
				return
			}
			pattern := `^HMAC-SHA256 apiKey=.+, date=.+, salt=[0-9a-f]{40}, signature=[0-9a-f]{64}$`
			matched, err := regexp.MatchString(pattern, result)
			if err != nil {
				errs <- fmt.Errorf("regex error: %w", err)
				return
			}
			if !matched {
				errs <- fmt.Errorf("format mismatch: %q", result)
			}
		}()
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestSign_EmptyInputs(t *testing.T) {
	tests := []struct {
		name   string
		data   string
		secret string
	}{
		{"both empty", "", ""},
		{"data only", "data", ""},
		{"secret only", "", "secret"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sign(tt.data, tt.secret)
			// HMAC-SHA256 always produces 64 hex chars (32 bytes)
			if len(result) != 64 {
				t.Errorf("expected 64 hex chars, got %d", len(result))
			}

			// Verify result matches independent computation
			h := hmac.New(sha256.New, []byte(tt.secret))
			h.Write([]byte(tt.data))
			expected := hex.EncodeToString(h.Sum(nil))
			if result != expected {
				t.Errorf("sign(%q, %q) = %q, want %q", tt.data, tt.secret, result, expected)
			}
		})
	}
}
