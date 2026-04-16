// Package client provides a REST HTTP client for the SOLAPI API.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/solapi/solactl/pkg/apierror"
	"github.com/solapi/solactl/pkg/auth"
	"github.com/solapi/solactl/pkg/logger"
)

// BaseURL is the fixed SOLAPI API endpoint.
const BaseURL = "https://api.solapi.com"

// sensitiveFields is the set of JSON field names whose values must be
// redacted in debug logs. Covers apiKey, apiSecret, senderKey(s),
// groupKey(s), secretKey. Values of any type (string, array, object)
// are replaced with "[REDACTED]".
var sensitiveFields = map[string]bool{
	"apiKey":     true,
	"apiSecret":  true,
	"senderKey":  true,
	"senderKeys": true,
	"groupKey":   true,
	"groupKeys":  true,
	"secretKey":  true,
}

// redactSensitiveFields replaces known sensitive JSON field values with "[REDACTED]".
// Uses JSON decoding to handle all value types (strings, arrays, objects).
// Supports both top-level objects and top-level arrays.
func redactSensitiveFields(s string) string {
	var raw any
	if json.Unmarshal([]byte(s), &raw) != nil {
		return s // not valid JSON, return as-is
	}
	redactAny(raw)
	out, err := json.Marshal(raw)
	if err != nil {
		return s
	}
	return string(out)
}

func redactAny(v any) {
	switch val := v.(type) {
	case map[string]any:
		redactMap(val)
	case []any:
		redactSlice(val)
	}
}

func redactMap(m map[string]any) {
	for k, v := range m {
		if sensitiveFields[k] {
			m[k] = "[REDACTED]"
			continue
		}
		switch val := v.(type) {
		case map[string]any:
			redactMap(val)
		case []any:
			redactSlice(val)
		}
	}
}

func redactSlice(s []any) {
	for _, item := range s {
		if m, ok := item.(map[string]any); ok {
			redactMap(m)
		}
	}
}

// Client is an HTTP client for SOLAPI REST endpoints.
type Client struct {
	HTTPClient      *http.Client
	APIKey          string
	APISecret       string
	MaxRetries      int
	BaseDelay       time.Duration
	BaseURLOverride string // If set, used instead of the BaseURL constant.
	UserAgent       string // User-Agent header value. Set by caller.
}

// baseURL returns the effective base URL for API requests.
func (c *Client) baseURL() string {
	if c.BaseURLOverride != "" {
		return c.BaseURLOverride
	}
	return BaseURL
}

// New creates a Client with default settings.
func New(apiKey, apiSecret string) *Client {
	return &Client{
		HTTPClient: http.DefaultClient,
		APIKey:     apiKey,
		APISecret:  apiSecret,
		MaxRetries: 3,
		BaseDelay:  500 * time.Millisecond,
	}
}

// Get sends a GET request to the given API path.
func (c *Client) Get(ctx context.Context, path string, params url.Values) (json.RawMessage, error) {
	u := c.baseURL() + "/" + strings.TrimLeft(path, "/")
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	return c.executeWithRetry(ctx, http.MethodGet, u, nil, isRetryableGET)
}

// Post sends a POST request with a JSON body.
func (c *Client) Post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	u := c.baseURL() + "/" + strings.TrimLeft(path, "/")
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("JSON 직렬화 실패: %w", err)
	}
	return c.executeWithRetry(ctx, http.MethodPost, u, data, isRetryableMutation)
}

// Put sends a PUT request with a JSON body.
func (c *Client) Put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	u := c.baseURL() + "/" + strings.TrimLeft(path, "/")
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("JSON 직렬화 실패: %w", err)
	}
	return c.executeWithRetry(ctx, http.MethodPut, u, data, isRetryableMutation)
}

// Delete sends a DELETE request.
func (c *Client) Delete(ctx context.Context, path string) (json.RawMessage, error) {
	u := c.baseURL() + "/" + strings.TrimLeft(path, "/")
	return c.executeWithRetry(ctx, http.MethodDelete, u, nil, isRetryableMutation)
}

func (c *Client) executeWithRetry(ctx context.Context, method, rawURL string, body []byte, retryable func(error) bool) (json.RawMessage, error) {
	var lastErr error

	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		// Short-circuit if context is already expired
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if attempt > 0 {
			delay := c.BaseDelay * time.Duration(1<<(attempt-1))
			var jitter time.Duration
			if n := int64(delay) / 4; n > 0 {
				jitter = time.Duration(rand.Int64N(n))
			}
			wait := delay + jitter
			logger.Debug("재시�� %d/%d (대기: %v)", attempt, c.MaxRetries, wait)

			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}

		result, err := c.doRequest(ctx, method, rawURL, body)
		if err == nil {
			return result, nil
		}
		lastErr = err

		if !retryable(err) {
			return nil, err
		}
		logger.Debug("일시적 오류 (attempt %d/%d): %v", attempt+1, c.MaxRetries+1, err)
	}

	return nil, lastErr
}

func (c *Client) doRequest(ctx context.Context, method, rawURL string, body []byte) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	authHeader, err := auth.GenerateAuthorization(c.APIKey, c.APISecret)
	if err != nil {
		return nil, fmt.Errorf("generating authorization: %w", err)
	}
	req.Header.Set("Authorization", authHeader)

	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	} else {
		req.Header.Set("User-Agent", "solactl/unknown ("+runtime.GOOS+"/"+runtime.GOARCH+")")
	}

	logger.Debug("--> %s %s", method, rawURL)

	start := time.Now()
	resp, err := c.HTTPClient.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		logger.Debug("<-- ERROR (%v): %v", elapsed, err)
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	logger.Debug("<-- %d %s (%v)", resp.StatusCode, resp.Status, elapsed)
	if logger.IsEnabled() && len(respBody) > 0 {
		// Redact before truncation to prevent partial secret exposure at the cut point
		preview := redactSensitiveFields(string(respBody))
		runes := []rune(preview)
		if len(runes) > 500 {
			preview = string(runes[:500]) + "..."
		}
		logger.Debug("    body: %s", preview)
	}

	if resp.StatusCode >= 400 {
		return nil, parseErrorResponse(resp.StatusCode, respBody)
	}

	if len(respBody) == 0 {
		return json.RawMessage("null"), nil
	}
	return json.RawMessage(respBody), nil
}

// parseErrorResponse extracts an APIError from the response body.
func parseErrorResponse(statusCode int, body []byte) error {
	apiErr := &apierror.APIError{HTTPStatus: statusCode}

	var parsed struct {
		ErrorCode    string `json:"errorCode"`
		ErrorMessage string `json:"errorMessage"`
	}
	if json.Unmarshal(body, &parsed) == nil && (parsed.ErrorCode != "" || parsed.ErrorMessage != "") {
		apiErr.ErrorCode = parsed.ErrorCode
		apiErr.ErrorMessage = parsed.ErrorMessage
	} else {
		apiErr.ErrorMessage = http.StatusText(statusCode)
	}

	return apiErr
}

// isRetryableGET returns true for transient errors on GET requests.
func isRetryableGET(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *apierror.APIError
	if errors.As(err, &apiErr) {
		return apiErr.HTTPStatus >= 500 || apiErr.HTTPStatus == 429
	}
	return isNetworkError(err)
}

// isRetryableMutation only retries on 429 or pre-connect failures.
func isRetryableMutation(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *apierror.APIError
	if errors.As(err, &apiErr) {
		return apiErr.HTTPStatus == 429
	}
	msg := err.Error()
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host")
}

func isNetworkError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "EOF")
}
