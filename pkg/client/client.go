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
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
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
	HTTPClient        *http.Client
	APIKey            string
	APISecret         string
	MaxRetries        int
	BaseDelay         time.Duration
	BaseURLOverride   string // If set, used instead of the BaseURL constant.
	UserAgent         string // User-Agent header value. Set by caller.
	SkipAuthorization bool   // If true, do not attach SOLAPI HMAC Authorization.
}

// MultipartField is one regular form field in a multipart request.
type MultipartField struct {
	Name  string
	Value string
}

// MultipartFile is the file part in a multipart request.
type MultipartFile struct {
	FieldName   string
	Path        string
	FileName    string
	ContentType string
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
	data, err := marshalBody(body)
	if err != nil {
		return nil, fmt.Errorf("JSON 직렬화 실패: %w", err)
	}
	return c.executeWithRetry(ctx, http.MethodPost, u, data, isRetryableMutation)
}

// PostMultipart sends a POST request with multipart/form-data.
func (c *Client) PostMultipart(ctx context.Context, path string, fields []MultipartField, file MultipartFile) (json.RawMessage, error) {
	u := c.baseURL() + "/" + strings.TrimLeft(path, "/")
	data, contentType, err := buildMultipartBody(fields, file)
	if err != nil {
		return nil, err
	}
	return c.executeWithRetryContentType(ctx, http.MethodPost, u, data, contentType, isRetryableMutation)
}

// Put sends a PUT request with a JSON body.
func (c *Client) Put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	u := c.baseURL() + "/" + strings.TrimLeft(path, "/")
	data, err := marshalBody(body)
	if err != nil {
		return nil, fmt.Errorf("JSON 직렬화 실패: %w", err)
	}
	return c.executeWithRetry(ctx, http.MethodPut, u, data, isRetryableMutation)
}

// Patch sends a PATCH request with a JSON body. A nil body sends no payload.
func (c *Client) Patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	u := c.baseURL() + "/" + strings.TrimLeft(path, "/")
	data, err := marshalBody(body)
	if err != nil {
		return nil, fmt.Errorf("JSON 직렬화 실패: %w", err)
	}
	return c.executeWithRetry(ctx, http.MethodPatch, u, data, isRetryableMutation)
}

// Delete sends a DELETE request.
func (c *Client) Delete(ctx context.Context, path string) (json.RawMessage, error) {
	u := c.baseURL() + "/" + strings.TrimLeft(path, "/")
	return c.executeWithRetry(ctx, http.MethodDelete, u, nil, isRetryableMutation)
}

func marshalBody(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	return json.Marshal(body)
}

func buildMultipartBody(fields []MultipartField, file MultipartFile) ([]byte, string, error) {
	if file.FieldName == "" {
		return nil, "", errors.New("multipart 파일 필드명이 비어 있습니다")
	}
	if file.Path == "" {
		return nil, "", errors.New("multipart 파일 경로가 비어 있습니다")
	}

	f, err := os.Open(file.Path)
	if err != nil {
		return nil, "", fmt.Errorf("파일 열기 실패: %w", err)
	}
	defer func() { _ = f.Close() }()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for _, field := range fields {
		if err := writer.WriteField(field.Name, field.Value); err != nil {
			_ = writer.Close()
			return nil, "", fmt.Errorf("multipart 필드 작성 실패: %w", err)
		}
	}

	filename := file.FileName
	if filename == "" {
		filename = filepath.Base(file.Path)
	}
	contentType := file.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, multipartEscape(file.FieldName), multipartEscape(filename)))
	header.Set("Content-Type", contentType)

	part, err := writer.CreatePart(header)
	if err != nil {
		_ = writer.Close()
		return nil, "", fmt.Errorf("multipart 파일 파트 생성 실패: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		_ = writer.Close()
		return nil, "", fmt.Errorf("multipart 파일 복사 실패: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("multipart 종료 실패: %w", err)
	}
	return buf.Bytes(), writer.FormDataContentType(), nil
}

func multipartEscape(s string) string {
	return strings.NewReplacer("\\", "\\\\", `"`, `\"`, "\r", "", "\n", "").Replace(s)
}

func (c *Client) executeWithRetry(ctx context.Context, method, rawURL string, body []byte, retryable func(error) bool) (json.RawMessage, error) {
	contentType := ""
	if body != nil {
		contentType = "application/json"
	}
	return c.executeWithRetryContentType(ctx, method, rawURL, body, contentType, retryable)
}

func (c *Client) executeWithRetryContentType(ctx context.Context, method, rawURL string, body []byte, contentType string, retryable func(error) bool) (json.RawMessage, error) {
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
			logger.Debug("재시도 %d/%d (대기: %v)", attempt, c.MaxRetries, wait)

			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}

		result, err := c.doRequest(ctx, method, rawURL, body, contentType)
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

func (c *Client) doRequest(ctx context.Context, method, rawURL string, body []byte, contentType string) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if body != nil && contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	if !c.SkipAuthorization {
		authHeader, err := auth.GenerateAuthorization(c.APIKey, c.APISecret)
		if err != nil {
			return nil, fmt.Errorf("generating authorization: %w", err)
		}
		req.Header.Set("Authorization", authHeader)
	}

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
		Message      string `json:"message"`
		Error        string `json:"error"`
	}
	if json.Unmarshal(body, &parsed) == nil && (parsed.ErrorCode != "" || parsed.ErrorMessage != "" || parsed.Message != "" || parsed.Error != "") {
		apiErr.ErrorCode = parsed.ErrorCode
		switch {
		case parsed.ErrorMessage != "":
			apiErr.ErrorMessage = parsed.ErrorMessage
		case parsed.Message != "":
			apiErr.ErrorMessage = parsed.Message
		default:
			apiErr.ErrorMessage = parsed.Error
		}
		enrichPlanError(apiErr, body)
	} else {
		apiErr.ErrorMessage = http.StatusText(statusCode)
		enrichPlanError(apiErr, body)
	}

	return apiErr
}

func enrichPlanError(apiErr *apierror.APIError, body []byte) {
	var raw map[string]any
	if json.Unmarshal(body, &raw) != nil {
		return
	}
	payload := raw
	if nested, ok := raw["message"].(map[string]any); ok {
		payload = nested
	}

	if code := stringField(payload, "errorCode"); code != "" && apiErr.ErrorCode == "" {
		apiErr.ErrorCode = code
	}
	if message := stringField(payload, "message"); message != "" {
		apiErr.ErrorMessage = message
	}

	switch apiErr.ErrorCode {
	case "PlanQuotaExceeded":
		apiErr.ErrorMessage = formatPlanQuotaExceeded(apiErr.ErrorMessage, payload)
	case "PlanFeatureDisabled":
		apiErr.ErrorMessage = formatPlanFeatureDisabled(apiErr.ErrorMessage, payload)
	}
}

func formatPlanQuotaExceeded(base string, payload map[string]any) string {
	parts := []string{}
	if base != "" {
		parts = append(parts, base)
	}
	label := firstNonEmpty(stringField(payload, "dimensionLabel"), stringField(payload, "dimension"))
	usage, hasUsage := numberField(payload, "usage")
	limit, hasLimit := numberField(payload, "limit")
	if label != "" && hasUsage && hasLimit {
		parts = append(parts, fmt.Sprintf("현재 사용량: %s %s/%s", label, formatNumber(usage), formatNumber(limit)))
	} else if label != "" {
		parts = append(parts, "제한 항목: "+label)
	}
	nextTier := stringField(payload, "nextTier")
	nextTierLimit, hasNextTierLimit := numberField(payload, "nextTierLimit")
	if nextTier != "" && hasNextTierLimit {
		parts = append(parts, fmt.Sprintf("권장 플랜: %s (한도 %s)", nextTier, formatNumber(nextTierLimit)))
	} else if nextTier != "" {
		parts = append(parts, "권장 플랜: "+nextTier)
	}
	return strings.Join(parts, ". ")
}

func formatPlanFeatureDisabled(base string, payload map[string]any) string {
	parts := []string{}
	if base != "" {
		parts = append(parts, base)
	}
	label := firstNonEmpty(stringField(payload, "dimensionLabel"), stringField(payload, "feature"), stringField(payload, "dimension"))
	if label != "" {
		parts = append(parts, "제한 기능: "+label)
	}
	if nextTier := stringField(payload, "nextTier"); nextTier != "" {
		parts = append(parts, "사용 가능한 플랜: "+nextTier)
	}
	return strings.Join(parts, ". ")
}

func stringField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	default:
		return fmt.Sprint(val)
	}
}

func numberField(m map[string]any, key string) (float64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case json.Number:
		n, err := val.Float64()
		return n, err == nil
	default:
		return 0, false
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func formatNumber(n float64) string {
	if n == float64(int64(n)) {
		return fmt.Sprintf("%.0f", n)
	}
	return fmt.Sprintf("%g", n)
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
