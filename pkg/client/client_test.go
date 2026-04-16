package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/solapi/solactl/pkg/apierror"
)

// newTestServer creates an httptest server and returns a Client pointing to it.
func newTestServer(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	c := &Client{
		HTTPClient: ts.Client(),
		APIKey:     "testkey",
		APISecret:  "testsecret",
		MaxRetries: 0,
		BaseDelay:  time.Millisecond,
	}
	return c, ts
}

// directGet bypasses BaseURL and calls the test server directly.
func directGet(ctx context.Context, c *Client, rawURL string, params url.Values) (json.RawMessage, error) {
	u := rawURL
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	return c.executeWithRetry(ctx, http.MethodGet, u, nil, isRetryableGET)
}

func directPost(ctx context.Context, c *Client, rawURL string, body any) (json.RawMessage, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return c.executeWithRetry(ctx, http.MethodPost, rawURL, data, isRetryableMutation)
}

func directPut(ctx context.Context, c *Client, rawURL string, body any) (json.RawMessage, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return c.executeWithRetry(ctx, http.MethodPut, rawURL, data, isRetryableMutation)
}

func directDelete(ctx context.Context, c *Client, rawURL string) (json.RawMessage, error) {
	return c.executeWithRetry(ctx, http.MethodDelete, rawURL, nil, isRetryableMutation)
}

func TestGet_Success(t *testing.T) {
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"result":"ok"}`))
	})

	result, err := directGet(context.Background(), c, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]string
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed["result"] != "ok" {
		t.Errorf("got %v", parsed)
	}
}

func TestGet_WithParams(t *testing.T) {
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") != "value" {
			t.Errorf("missing query param: %s", r.URL.RawQuery)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	})

	params := url.Values{"key": {"value"}}
	_, err := directGet(context.Background(), c, ts.URL+"/test", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPost_Success(t *testing.T) {
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content-type: %s", ct)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"created":true}`))
	})

	body := map[string]string{"name": "test"}
	result, err := directPost(context.Background(), c, ts.URL+"/test", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(result), "created") {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestPut_Success(t *testing.T) {
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"updated":true}`))
	})

	result, err := directPut(context.Background(), c, ts.URL+"/test", map[string]string{"status": "ACTIVE"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(result), "updated") {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestDelete_Success(t *testing.T) {
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"deleted":true}`))
	})

	result, err := directDelete(context.Background(), c, ts.URL+"/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(result), "deleted") {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestGet_EmptyBody(t *testing.T) {
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	result, err := directGet(context.Background(), c, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != "null" {
		t.Errorf("expected null for empty body, got %s", result)
	}
}

func TestGet_400_APIError(t *testing.T) {
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"errorCode":"ValidationError","errorMessage":"잘못된 요청"}`))
	})

	_, err := directGet(context.Background(), c, ts.URL+"/test", nil)
	if err == nil {
		t.Fatal("expected error")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.HTTPStatus != 400 {
		t.Errorf("status: got %d, want 400", apiErr.HTTPStatus)
	}
	if apiErr.ErrorCode != "ValidationError" {
		t.Errorf("code: got %s", apiErr.ErrorCode)
	}
}

func TestGet_401_APIError(t *testing.T) {
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"errorCode":"Unauthorized","errorMessage":"인증 실패"}`))
	})

	_, err := directGet(context.Background(), c, ts.URL+"/test", nil)
	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.HTTPStatus != 401 {
		t.Errorf("status: got %d", apiErr.HTTPStatus)
	}
}

func TestGet_500_NonJSON(t *testing.T) {
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(502)
		_, _ = w.Write([]byte(`<html>Bad Gateway</html>`))
	})

	_, err := directGet(context.Background(), c, ts.URL+"/test", nil)
	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.HTTPStatus != 502 {
		t.Errorf("status: got %d", apiErr.HTTPStatus)
	}
	if apiErr.ErrorMessage != "Bad Gateway" {
		t.Errorf("message: got %q", apiErr.ErrorMessage)
	}
}

func TestGet_Retry_5xx(t *testing.T) {
	var calls atomic.Int32
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n <= 2 {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"errorCode":"InternalServerError","errorMessage":"서버 오류"}`))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	c.MaxRetries = 3
	c.BaseDelay = time.Millisecond

	result, err := directGet(context.Background(), c, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if !strings.Contains(string(result), "ok") {
		t.Errorf("unexpected: %s", result)
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", calls.Load())
	}
}

func TestGet_Retry_429(t *testing.T) {
	var calls atomic.Int32
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"errorCode":"TooManyRequests","errorMessage":"요청 한도 초과"}`))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	})
	c.MaxRetries = 2
	c.BaseDelay = time.Millisecond

	_, err := directGet(context.Background(), c, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if calls.Load() != 2 {
		t.Errorf("expected 2 calls, got %d", calls.Load())
	}
}

func TestPost_NoRetry_5xx(t *testing.T) {
	var calls atomic.Int32
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"errorCode":"InternalServerError","errorMessage":"서버 오류"}`))
	})
	c.MaxRetries = 3
	c.BaseDelay = time.Millisecond

	_, err := directPost(context.Background(), c, ts.URL+"/test", map[string]string{})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 1 {
		t.Errorf("POST should not retry on 5xx; calls: %d", calls.Load())
	}
}

func TestPost_Retry_429(t *testing.T) {
	var calls atomic.Int32
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"errorCode":"TooManyRequests","errorMessage":"요청 한도 초과"}`))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	})
	c.MaxRetries = 2
	c.BaseDelay = time.Millisecond

	_, err := directPost(context.Background(), c, ts.URL+"/test", map[string]string{})
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if calls.Load() != 2 {
		t.Errorf("expected 2 calls, got %d", calls.Load())
	}
}

func TestContextCancellation(t *testing.T) {
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(200)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := directGet(ctx, c, ts.URL+"/test", nil)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestContextCancellation_DuringRetryWait(t *testing.T) {
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"errorCode":"InternalServerError","errorMessage":"err"}`))
	})
	c.MaxRetries = 5
	c.BaseDelay = 5 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := directGet(ctx, c, ts.URL+"/test", nil)
	if err == nil {
		t.Fatal("expected context error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got: %v", err)
	}
}

func TestNew_Defaults(t *testing.T) {
	c := New("key", "secret")
	if c.APIKey != "key" {
		t.Errorf("APIKey: %s", c.APIKey)
	}
	if c.APISecret != "secret" {
		t.Errorf("APISecret: %s", c.APISecret)
	}
	if c.MaxRetries != 3 {
		t.Errorf("MaxRetries: %d", c.MaxRetries)
	}
	if c.BaseDelay != 500*time.Millisecond {
		t.Errorf("BaseDelay: %v", c.BaseDelay)
	}
	if c.HTTPClient == nil {
		t.Error("HTTPClient is nil")
	}
}

func TestIsRetryableGET(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"500", &apierror.APIError{HTTPStatus: 500}, true},
		{"429", &apierror.APIError{HTTPStatus: 429}, true},
		{"400", &apierror.APIError{HTTPStatus: 400}, false},
		{"401", &apierror.APIError{HTTPStatus: 401}, false},
		{"connection refused", errors.New("connection refused"), true},
		{"no such host", errors.New("no such host"), true},
		{"EOF", errors.New("unexpected EOF"), true},
		{"i/o timeout", errors.New("i/o timeout"), true},
		{"random error", errors.New("something"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableGET(tt.err)
			if got != tt.want {
				t.Errorf("isRetryableGET(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsRetryableMutation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"429", &apierror.APIError{HTTPStatus: 429}, true},
		{"500", &apierror.APIError{HTTPStatus: 500}, false},
		{"connection refused", errors.New("connection refused"), true},
		{"no such host", errors.New("no such host"), true},
		{"EOF", errors.New("unexpected EOF"), false},
		{"i/o timeout", errors.New("i/o timeout"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableMutation(tt.err)
			if got != tt.want {
				t.Errorf("isRetryableMutation(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestAuthorizationHeader_Present(t *testing.T) {
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "HMAC-SHA256 ") {
			t.Errorf("auth header: %q", auth)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	})

	_, err := directGet(context.Background(), c, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAllRetries_Exhausted(t *testing.T) {
	var calls atomic.Int32
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"errorCode":"InternalServerError","errorMessage":"err"}`))
	})
	c.MaxRetries = 2
	c.BaseDelay = time.Millisecond

	_, err := directGet(context.Background(), c, ts.URL+"/test", nil)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	// 1 initial + 2 retries = 3 total
	if calls.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", calls.Load())
	}
}

func TestGet_MaxRetries_Zero(t *testing.T) {
	var calls atomic.Int32
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"errorCode":"InternalServerError","errorMessage":"err"}`))
	})
	c.MaxRetries = 0
	c.BaseDelay = time.Millisecond

	_, err := directGet(context.Background(), c, ts.URL+"/test", nil)
	if err == nil {
		t.Fatal("expected error with MaxRetries=0")
	}
	if calls.Load() != 1 {
		t.Errorf("MaxRetries=0 should attempt exactly 1 call, got %d", calls.Load())
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.HTTPStatus != 500 {
		t.Errorf("status: got %d, want 500", apiErr.HTTPStatus)
	}
}

func TestGet_BaseDelay_Zero(t *testing.T) {
	var calls atomic.Int32
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n <= 2 {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"errorCode":"InternalServerError","errorMessage":"err"}`))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	c.MaxRetries = 3
	c.BaseDelay = 0 // zero delay — jitter n = int64(0)/4 = 0, rand.Int64N not called

	result, err := directGet(context.Background(), c, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("expected success with BaseDelay=0, got: %v", err)
	}
	if !strings.Contains(string(result), "ok") {
		t.Errorf("unexpected result: %s", result)
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", calls.Load())
	}
}

func TestGet_NthCallFailure(t *testing.T) {
	var calls atomic.Int32
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n <= 2 {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		// 3rd call returns 500
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"errorCode":"InternalServerError","errorMessage":"err"}`))
	})
	c.MaxRetries = 0 // no retries
	c.BaseDelay = time.Millisecond

	// Call 1: success
	result1, err := directGet(context.Background(), c, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("call 1 should succeed: %v", err)
	}
	if result1 == nil {
		t.Error("call 1 result should not be nil")
	}

	// Call 2: success
	result2, err := directGet(context.Background(), c, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("call 2 should succeed: %v", err)
	}
	if result2 == nil {
		t.Error("call 2 result should not be nil")
	}

	// Call 3: failure
	_, err = directGet(context.Background(), c, ts.URL+"/test", nil)
	if err == nil {
		t.Fatal("call 3 should fail")
	}
	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.HTTPStatus != 500 {
		t.Errorf("expected 500, got %d", apiErr.HTTPStatus)
	}
}

func TestPost_MarshalError(t *testing.T) {
	c := New("key", "secret")
	// func values cannot be marshaled to JSON
	_, err := c.Post(context.Background(), "/test", func() {})
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if !strings.Contains(err.Error(), "JSON 직렬화 실패") {
		t.Errorf("expected 'JSON 직렬화 실패' in error, got: %v", err)
	}
}

func TestGet_ResponseBodyPreview_Exactly500Chars(t *testing.T) {
	// Body of exactly 500 runes should not be truncated
	body500 := strings.Repeat("가", 500) // 500 runes (1500 bytes)
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`"` + body500 + `"`))
	})

	result, err := directGet(context.Background(), c, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Result should contain the full body without truncation
	var s string
	if err := json.Unmarshal(result, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len([]rune(s)) != 500 {
		t.Errorf("expected 500 runes, got %d", len([]rune(s)))
	}
}

func TestGet_ResponseBodyPreview_Over500Chars(t *testing.T) {
	// Body of 501 runes — the response data itself is fine, preview truncation
	// is debug-only. The actual response should still be complete.
	body501 := strings.Repeat("나", 501) // 501 runes
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`"` + body501 + `"`))
	})

	result, err := directGet(context.Background(), c, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Even though debug preview truncates at 500 runes, actual response
	// must still contain the full data.
	var s string
	if err := json.Unmarshal(result, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len([]rune(s)) != 501 {
		t.Errorf("expected 501 runes, got %d", len([]rune(s)))
	}
}

func TestParseErrorResponse_EmptyBody(t *testing.T) {
	err := parseErrorResponse(502, []byte{})
	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	// Empty body should fallback to http.StatusText
	if apiErr.ErrorMessage != http.StatusText(http.StatusBadGateway) {
		t.Errorf("expected %q, got %q", http.StatusText(http.StatusBadGateway), apiErr.ErrorMessage)
	}
	if apiErr.ErrorCode != "" {
		t.Errorf("expected empty errorCode, got %q", apiErr.ErrorCode)
	}
}

func TestParseErrorResponse_NullJSON(t *testing.T) {
	err := parseErrorResponse(404, []byte("null"))
	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	// "null" JSON unmarshals successfully but yields empty strings,
	// so should fallback to http.StatusText.
	if apiErr.ErrorMessage != http.StatusText(404) {
		t.Errorf("expected %q, got %q", http.StatusText(404), apiErr.ErrorMessage)
	}
}

func TestGet_Concurrent(t *testing.T) {
	var calls atomic.Int32
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	c.MaxRetries = 0

	const goroutines = 20
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	results := make([]json.RawMessage, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = directGet(context.Background(), c, ts.URL+"/test", nil)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: unexpected error: %v", i, err)
		}
	}
	for i, r := range results {
		if !strings.Contains(string(r), "ok") {
			t.Errorf("goroutine %d: unexpected result: %s", i, r)
		}
	}
	if calls.Load() != int32(goroutines) {
		t.Errorf("expected %d calls, got %d", goroutines, calls.Load())
	}
}

func FuzzParseErrorResponse(f *testing.F) {
	f.Add(400, []byte(`{"errorCode":"Bad","errorMessage":"bad"}`))
	f.Add(500, []byte(`<html>error</html>`))
	f.Add(200, []byte{})
	f.Add(404, []byte("null"))
	f.Add(502, []byte(`{`))

	f.Fuzz(func(t *testing.T, statusCode int, body []byte) {
		// Must never panic regardless of input
		err := parseErrorResponse(statusCode, body)
		if err == nil {
			t.Error("parseErrorResponse should always return non-nil error")
		}
	})
}

func TestPut_NoRetry_5xx(t *testing.T) {
	var calls atomic.Int32
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"errorCode":"InternalServerError","errorMessage":"서버 오류"}`))
	})
	c.MaxRetries = 3
	c.BaseDelay = time.Millisecond

	_, err := directPut(context.Background(), c, ts.URL+"/test", map[string]string{"k": "v"})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 1 {
		t.Errorf("PUT should not retry on 5xx; calls: %d", calls.Load())
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.HTTPStatus != 500 {
		t.Errorf("status: got %d, want 500", apiErr.HTTPStatus)
	}
}

func TestDelete_404(t *testing.T) {
	c, ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"errorCode":"ResourceNotFound","errorMessage":"리소스를 찾을 수 없습니다"}`))
	})

	_, err := directDelete(context.Background(), c, ts.URL+"/test")
	if err == nil {
		t.Fatal("expected error for 404")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.HTTPStatus != 404 {
		t.Errorf("status: got %d, want 404", apiErr.HTTPStatus)
	}
	if apiErr.ErrorCode != "ResourceNotFound" {
		t.Errorf("code: got %q, want ResourceNotFound", apiErr.ErrorCode)
	}
	if apiErr.ErrorMessage != "리소스를 찾을 수 없습니다" {
		t.Errorf("message: got %q", apiErr.ErrorMessage)
	}
}

func TestBaseURL_DefaultAndOverride(t *testing.T) {
	c := New("key", "secret")

	// Default: uses constant BaseURL
	if got := c.baseURL(); got != BaseURL {
		t.Errorf("default baseURL: got %q, want %q", got, BaseURL)
	}

	// Override: uses BaseURLOverride
	c.BaseURLOverride = "http://custom:9999"
	if got := c.baseURL(); got != "http://custom:9999" {
		t.Errorf("override baseURL: got %q, want %q", got, "http://custom:9999")
	}

	// Clear override: back to default
	c.BaseURLOverride = ""
	if got := c.baseURL(); got != BaseURL {
		t.Errorf("cleared baseURL: got %q, want %q", got, BaseURL)
	}
}

func TestRedactSensitiveFields(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		mustContain    []string
		mustNotContain []string
	}{
		{
			name:           "senderKeys string redacted",
			input:          `{"channelId":"CH01","senderKeys":"secret123"}`,
			mustContain:    []string{`"senderKeys":"[REDACTED]"`, `"channelId":"CH01"`},
			mustNotContain: []string{"secret123"},
		},
		{
			name:           "senderKey singular redacted",
			input:          `{"senderKey":"abc-def-123"}`,
			mustContain:    []string{`"senderKey":"[REDACTED]"`},
			mustNotContain: []string{"abc-def-123"},
		},
		{
			name:           "groupKeys redacted",
			input:          `{"name":"test","groupKeys":"gk-secret"}`,
			mustContain:    []string{`"groupKeys":"[REDACTED]"`, `"name":"test"`},
			mustNotContain: []string{"gk-secret"},
		},
		{
			name:           "groupKey singular redacted",
			input:          `{"groupKey":"gk-single"}`,
			mustContain:    []string{`"groupKey":"[REDACTED]"`},
			mustNotContain: []string{"gk-single"},
		},
		{
			name:           "secretKey redacted",
			input:          `{"secretKey":"very-secret-value"}`,
			mustContain:    []string{`"secretKey":"[REDACTED]"`},
			mustNotContain: []string{"very-secret-value"},
		},
		{
			name:           "apiSecret redacted",
			input:          `{"apiSecret":"my-api-secret-123"}`,
			mustContain:    []string{`"apiSecret":"[REDACTED]"`},
			mustNotContain: []string{"my-api-secret-123"},
		},
		{
			name:           "apiKey redacted",
			input:          `{"apiKey":"NCSABC123","apiSecret":"secret456"}`,
			mustContain:    []string{`"apiKey":"[REDACTED]"`, `"apiSecret":"[REDACTED]"`},
			mustNotContain: []string{"NCSABC123", "secret456"},
		},
		{
			name:           "multiple sensitive fields",
			input:          `{"senderKeys":"sk1","groupKeys":"gk1","name":"safe"}`,
			mustContain:    []string{`"senderKeys":"[REDACTED]"`, `"groupKeys":"[REDACTED]"`, `"name":"safe"`},
			mustNotContain: []string{"sk1", "gk1"},
		},
		{
			name:           "array value redacted",
			input:          `{"senderKeys":["key1","key2"],"name":"test"}`,
			mustContain:    []string{`"senderKeys":"[REDACTED]"`, `"name":"test"`},
			mustNotContain: []string{"key1", "key2"},
		},
		{
			name:           "object value redacted",
			input:          `{"groupKeys":{"g1":"v1","g2":"v2"},"channelId":"CH01"}`,
			mustContain:    []string{`"groupKeys":"[REDACTED]"`, `"channelId":"CH01"`},
			mustNotContain: []string{"v1", "v2"},
		},
		{
			name:           "nested sensitive fields redacted",
			input:          `{"channels":[{"name":"ch1","senderKey":"sk123"}]}`,
			mustContain:    []string{`"senderKey":"[REDACTED]"`, `"name":"ch1"`},
			mustNotContain: []string{"sk123"},
		},
		{
			name:        "no sensitive fields unchanged",
			input:       `{"channelId":"CH01","name":"test","status":"ACTIVE"}`,
			mustContain: []string{`"channelId":"CH01"`, `"name":"test"`, `"status":"ACTIVE"`},
		},
		{
			name:        "empty string unchanged",
			input:       "",
			mustContain: nil,
		},
		{
			name:        "non-JSON unchanged",
			input:       "not a json string",
			mustContain: []string{"not a json string"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactSensitiveFields(tt.input)
			for _, s := range tt.mustContain {
				if !strings.Contains(got, s) {
					t.Errorf("output should contain %q, got: %s", s, got)
				}
			}
			for _, s := range tt.mustNotContain {
				if strings.Contains(got, s) {
					t.Errorf("output should NOT contain %q, got: %s", s, got)
				}
			}
		})
	}
}
