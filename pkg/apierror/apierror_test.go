package apierror

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestClassify_APIErrors(t *testing.T) {
	tests := []struct {
		name         string
		err          *APIError
		wantCategory Category
		wantMessage  string
		wantHint     string
	}{
		{
			name:         "401 unauthorized",
			err:          &APIError{HTTPStatus: 401, ErrorCode: "Unauthorized", ErrorMessage: "invalid key"},
			wantCategory: CategoryAuth,
			wantMessage:  "인증에 실패했습니다",
			wantHint:     "API 키를 확인하세요: solactl configure show",
		},
		{
			name:         "403 forbidden",
			err:          &APIError{HTTPStatus: 403, ErrorCode: "Forbidden", ErrorMessage: "no access"},
			wantCategory: CategoryAuth,
			wantMessage:  "접근 권한이 없습니다",
			wantHint:     "API 키의 권한을 확인하세요",
		},
		{
			name:         "404 not found",
			err:          &APIError{HTTPStatus: 404, ErrorCode: "NotFound", ErrorMessage: "not found"},
			wantCategory: CategoryNotFound,
			wantMessage:  "리소스를 찾을 수 없습니다",
			wantHint:     "ID를 확인하세요",
		},
		{
			name:         "400 bad request with message",
			err:          &APIError{HTTPStatus: 400, ErrorCode: "BadRequest", ErrorMessage: "title is required"},
			wantCategory: CategoryValidation,
			wantMessage:  "입력값이 올바르지 않습니다",
			wantHint:     "title is required",
		},
		{
			name:         "400 bad request without message",
			err:          &APIError{HTTPStatus: 400, ErrorCode: "BadRequest"},
			wantCategory: CategoryValidation,
			wantMessage:  "입력값이 올바르지 않습니다",
			wantHint:     "",
		},
		{
			name:         "400 validation error by code",
			err:          &APIError{ErrorCode: "ValidationError", ErrorMessage: "invalid phone"},
			wantCategory: CategoryValidation,
			wantMessage:  "입력값이 올바르지 않습니다",
			wantHint:     "invalid phone",
		},
		{
			name:         "429 rate limit",
			err:          &APIError{HTTPStatus: 429, ErrorCode: "TooManyRequests", ErrorMessage: "rate limited"},
			wantCategory: CategoryRateLimit,
			wantMessage:  "요청 한도를 초과했습니다",
			wantHint:     "잠시 후 다시 시도하세요",
		},
		{
			name:         "500 server error",
			err:          &APIError{HTTPStatus: 500, ErrorCode: "InternalServerError", ErrorMessage: "oops"},
			wantCategory: CategoryServer,
			wantMessage:  "서버 오류가 발생했습니다",
			wantHint:     "잠시 후 다시 시도하세요",
		},
		{
			name:         "502 bad gateway",
			err:          &APIError{HTTPStatus: 502, ErrorCode: "BadGateway", ErrorMessage: "upstream down"},
			wantCategory: CategoryServer,
			wantMessage:  "서버 오류가 발생했습니다",
			wantHint:     "잠시 후 다시 시도하세요",
		},
		{
			name:         "unknown code by code string only (NotFound)",
			err:          &APIError{ErrorCode: "NotFound", ErrorMessage: "gone"},
			wantCategory: CategoryNotFound,
			wantMessage:  "리소스를 찾을 수 없습니다",
			wantHint:     "ID를 확인하세요",
		},
		{
			name:         "429 by code string only (no HTTPStatus)",
			err:          &APIError{ErrorCode: "TooManyRequests", ErrorMessage: "slow down"},
			wantCategory: CategoryRateLimit,
			wantMessage:  "요청 한도를 초과했습니다",
			wantHint:     "잠시 후 다시 시도하세요",
		},
		{
			name:         "500 by code string only (no HTTPStatus)",
			err:          &APIError{ErrorCode: "InternalServerError", ErrorMessage: "oops"},
			wantCategory: CategoryServer,
			wantMessage:  "서버 오류가 발생했습니다",
			wantHint:     "잠시 후 다시 시도하세요",
		},
		{
			name:         "completely unknown",
			err:          &APIError{HTTPStatus: 418, ErrorCode: "Teapot", ErrorMessage: "I'm a teapot"},
			wantCategory: CategoryUnknown,
			wantMessage:  "I'm a teapot",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.err)
			if got.Category != tt.wantCategory {
				t.Errorf("Category = %d, want %d", got.Category, tt.wantCategory)
			}
			if got.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", got.Message, tt.wantMessage)
			}
			if tt.wantHint != "" && got.Hint != tt.wantHint {
				t.Errorf("Hint = %q, want %q", got.Hint, tt.wantHint)
			}
		})
	}
}

func TestClassify_GenericErrors(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantCategory Category
		wantMessage  string
	}{
		{
			name:         "connection refused",
			err:          fmt.Errorf("dial tcp 127.0.0.1:7001: connection refused"),
			wantCategory: CategoryNetwork,
			wantMessage:  "서버에 연결할 수 없습니다",
		},
		{
			name:         "no such host",
			err:          fmt.Errorf("dial tcp: lookup unknown.host: no such host"),
			wantCategory: CategoryNetwork,
			wantMessage:  "서버 주소를 찾을 수 없습니다",
		},
		{
			name:         "context deadline exceeded",
			err:          fmt.Errorf("context deadline exceeded"),
			wantCategory: CategoryNetwork,
			wantMessage:  "요청 시간이 초과되었습니다",
		},
		{
			name:         "context canceled",
			err:          fmt.Errorf("context canceled"),
			wantCategory: CategoryUnknown,
			wantMessage:  "요청이 취소되었습니다",
		},
		{
			name:         "i/o timeout",
			err:          fmt.Errorf("read tcp: i/o timeout"),
			wantCategory: CategoryNetwork,
			wantMessage:  "네트워크 연결 시간이 초과되었습니다",
		},
		{
			name:         "connection reset",
			err:          fmt.Errorf("read tcp: connection reset by peer"),
			wantCategory: CategoryNetwork,
			wantMessage:  "서버와의 연결이 끊어졌습니다",
		},
		{
			name:         "unexpected EOF",
			err:          fmt.Errorf("unexpected EOF"),
			wantCategory: CategoryNetwork,
			wantMessage:  "서버와의 연결이 끊어졌습니다",
		},
		{
			name:         "unknown error",
			err:          fmt.Errorf("something completely unexpected"),
			wantCategory: CategoryUnknown,
			wantMessage:  "something completely unexpected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.err)
			if got.Category != tt.wantCategory {
				t.Errorf("Category = %d, want %d", got.Category, tt.wantCategory)
			}
			if got.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", got.Message, tt.wantMessage)
			}
		})
	}
}

func TestClassify_Nil(t *testing.T) {
	got := Classify(nil)
	if got != nil {
		t.Errorf("Classify(nil) = %v, want nil", got)
	}
}

func TestClassifiedError_Unwrap(t *testing.T) {
	orig := fmt.Errorf("original error")
	ce := &ClassifiedError{Original: orig, Category: CategoryUnknown, Message: "test"}

	unwrapped := ce.Unwrap()
	if unwrapped != orig {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, orig)
	}

	// errors.Is should work through Unwrap
	wrapped := fmt.Errorf("wrapped: %w", ce)
	if !errors.Is(wrapped, orig) {
		t.Error("errors.Is should find original through ClassifiedError")
	}
}

func TestClassifiedError_Error(t *testing.T) {
	orig := fmt.Errorf("original message")
	ce := &ClassifiedError{Original: orig, Category: CategoryAuth, Message: "인증 실패"}
	if ce.Error() != "original message" {
		t.Errorf("Error() = %q, want %q", ce.Error(), "original message")
	}
}

func TestClassify_WrappedAPIError(t *testing.T) {
	apiErr := &APIError{HTTPStatus: 401, ErrorCode: "Unauthorized", ErrorMessage: "bad"}
	wrapped := fmt.Errorf("HTTP request failed: %w", apiErr)

	got := Classify(wrapped)
	if got.Category != CategoryAuth {
		t.Errorf("Category = %d, want CategoryAuth (%d)", got.Category, CategoryAuth)
	}
}

func TestClassify_AllCategoriesHaveNonEmptyMessage(t *testing.T) {
	testErrors := []error{
		&APIError{HTTPStatus: 401, ErrorCode: "Unauthorized", ErrorMessage: "x"},
		&APIError{HTTPStatus: 403, ErrorCode: "Forbidden", ErrorMessage: "x"},
		&APIError{HTTPStatus: 404, ErrorCode: "NotFound", ErrorMessage: "x"},
		&APIError{HTTPStatus: 400, ErrorCode: "BadRequest", ErrorMessage: "x"},
		&APIError{HTTPStatus: 429, ErrorMessage: "x"},
		&APIError{HTTPStatus: 500, ErrorMessage: "x"},
		fmt.Errorf("connection refused"),
		fmt.Errorf("no such host"),
		fmt.Errorf("context deadline exceeded"),
		fmt.Errorf("unknown error type"),
	}

	for _, err := range testErrors {
		ce := Classify(err)
		if ce.Message == "" {
			t.Errorf("Classify(%v) has empty Message", err)
		}
	}
}

func TestClassify_APIError_AllFieldsEmpty(t *testing.T) {
	input := &APIError{}
	ce := Classify(input)

	if ce.Category != CategoryUnknown {
		t.Errorf("Category = %d, want CategoryUnknown (%d)", ce.Category, CategoryUnknown)
	}
	if ce.Message == "" {
		t.Error("Message should not be empty even for zero-valued APIError")
	}
	if ce.Message != "HTTP 0" {
		t.Errorf("Message = %q, want %q", ce.Message, "HTTP 0")
	}
}

func TestClassify_Unauthorized_ByCodeOnly(t *testing.T) {
	input := &APIError{ErrorCode: "Unauthorized", ErrorMessage: "bad creds"}
	ce := Classify(input)

	if ce.Category != CategoryAuth {
		t.Errorf("Category = %d, want CategoryAuth (%d)", ce.Category, CategoryAuth)
	}
}

func TestClassify_Forbidden_ByCodeOnly(t *testing.T) {
	input := &APIError{ErrorCode: "Forbidden", ErrorMessage: "no access"}
	ce := Classify(input)

	if ce.Category != CategoryAuth {
		t.Errorf("Category = %d, want CategoryAuth (%d)", ce.Category, CategoryAuth)
	}
}

func TestClassify_BadRequest_ByCodeOnly(t *testing.T) {
	input := &APIError{ErrorCode: "BadRequest", ErrorMessage: "need title"}
	ce := Classify(input)

	if ce.Category != CategoryValidation {
		t.Errorf("Category = %d, want CategoryValidation (%d)", ce.Category, CategoryValidation)
	}
	if ce.Hint != "need title" {
		t.Errorf("Hint = %q, want %q", ce.Hint, "need title")
	}
}

func TestAPIError_Error_WithErrorCode(t *testing.T) {
	e := &APIError{ErrorCode: "BadRequest", ErrorMessage: "invalid input"}
	if e.Error() != "BadRequest: invalid input" {
		t.Errorf("Error() = %q, want %q", e.Error(), "BadRequest: invalid input")
	}
}

func TestAPIError_Error_WithMessageOnly(t *testing.T) {
	e := &APIError{ErrorMessage: "something went wrong"}
	if e.Error() != "something went wrong" {
		t.Errorf("Error() = %q, want %q", e.Error(), "something went wrong")
	}
}

func TestAPIError_Error_StatusOnly(t *testing.T) {
	e := &APIError{HTTPStatus: 500}
	if e.Error() != "HTTP 500" {
		t.Errorf("Error() = %q, want %q", e.Error(), "HTTP 500")
	}
}

func TestAPIError_Error_Empty(t *testing.T) {
	e := &APIError{}
	if e.Error() != "HTTP 0" {
		t.Errorf("Error() = %q, want %q", e.Error(), "HTTP 0")
	}
}

func TestClassify_HintContainsSolactl(t *testing.T) {
	// Auth error hints should reference solactl, not colactl
	e := &APIError{HTTPStatus: 401, ErrorCode: "Unauthorized", ErrorMessage: "bad"}
	ce := Classify(e)
	if ce.Hint == "" {
		t.Fatal("expected non-empty hint for auth error")
	}
	if !contains(ce.Hint, "solactl") {
		t.Errorf("Hint should reference solactl, got: %q", ce.Hint)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestClassify_Idempotent(t *testing.T) {
	apiErr := &APIError{HTTPStatus: 429, ErrorCode: "TooManyRequests", ErrorMessage: "slow down"}

	first := Classify(apiErr)
	second := Classify(apiErr)

	if first.Category != second.Category {
		t.Errorf("Category mismatch: %d vs %d", first.Category, second.Category)
	}
	if first.Message != second.Message {
		t.Errorf("Message mismatch: %q vs %q", first.Message, second.Message)
	}
	if first.Hint != second.Hint {
		t.Errorf("Hint mismatch: %q vs %q", first.Hint, second.Hint)
	}
}

func TestClassify_HTTPStatusBoundaries(t *testing.T) {
	tests := []struct {
		status       int
		wantCategory Category
	}{
		{399, CategoryUnknown},
		{400, CategoryValidation},
		{401, CategoryAuth},
		{403, CategoryAuth},
		{404, CategoryNotFound},
		{405, CategoryUnknown},
		{429, CategoryRateLimit},
		{430, CategoryUnknown},
		{499, CategoryUnknown},
		{500, CategoryServer},
		{501, CategoryServer},
		{502, CategoryServer},
		{503, CategoryServer},
		{599, CategoryServer},
		{600, CategoryServer}, // >= 500 matches the server error branch
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.status), func(t *testing.T) {
			apiErr := &APIError{HTTPStatus: tt.status, ErrorMessage: "test"}
			ce := Classify(apiErr)

			if ce.Category != tt.wantCategory {
				t.Errorf("Classify(status=%d): Category = %d, want %d",
					tt.status, ce.Category, tt.wantCategory)
			}
			// Message must never be empty
			if ce.Message == "" {
				t.Errorf("Classify(status=%d): Message is empty", tt.status)
			}
		})
	}
}

func TestClassify_WrappedError(t *testing.T) {
	apiErr := &APIError{HTTPStatus: 403, ErrorCode: "Forbidden", ErrorMessage: "no access"}
	wrapped := fmt.Errorf("context: %w", apiErr)

	ce := Classify(wrapped)
	if ce == nil {
		t.Fatal("Classify returned nil for wrapped APIError")
	}
	if ce.Category != CategoryAuth {
		t.Errorf("Category = %d, want CategoryAuth (%d)", ce.Category, CategoryAuth)
	}
	if ce.Message != "접근 권한이 없습니다" {
		t.Errorf("Message = %q, want %q", ce.Message, "접근 권한이 없습니다")
	}

	// Verify the original APIError is reachable via errors.As
	var extracted *APIError
	if !errors.As(ce.Original, &extracted) {
		t.Error("expected to extract APIError from ClassifiedError.Original")
	}
}

func TestClassify_Concurrent(t *testing.T) {
	apiErr := &APIError{HTTPStatus: 500, ErrorCode: "InternalServerError", ErrorMessage: "oops"}

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	results := make([]*ClassifiedError, goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx] = Classify(apiErr)
		}(i)
	}
	wg.Wait()

	// All results should be identical
	first := results[0]
	for i := 1; i < goroutines; i++ {
		r := results[i]
		if r.Category != first.Category {
			t.Errorf("goroutine %d: Category = %d, want %d", i, r.Category, first.Category)
		}
		if r.Message != first.Message {
			t.Errorf("goroutine %d: Message = %q, want %q", i, r.Message, first.Message)
		}
		if r.Hint != first.Hint {
			t.Errorf("goroutine %d: Hint = %q, want %q", i, r.Hint, first.Hint)
		}
	}
}

func FuzzClassifyGeneric(f *testing.F) {
	f.Add("connection refused")
	f.Add("no such host")
	f.Add("context deadline exceeded")
	f.Add("context canceled")
	f.Add("i/o timeout")
	f.Add("EOF")
	f.Add("connection reset")
	f.Add("")
	f.Add("some random error message")

	f.Fuzz(func(t *testing.T, msg string) {
		// Must not panic regardless of input
		ce := Classify(errors.New(msg))
		if ce == nil {
			t.Error("Classify should never return nil for non-nil error")
		}
		// For the default branch, Message == err.Error(), which can be empty
		// when the original error message is empty. That is valid behavior.
		// For non-empty inputs, the message should always be non-empty.
		if msg != "" && ce.Message == "" {
			t.Errorf("Classify(errors.New(%q)): expected non-empty Message for non-empty input", msg)
		}
	})
}

func TestAPIError_Error_AllCombinations(t *testing.T) {
	tests := []struct {
		name         string
		err          APIError
		wantContains string
	}{
		{
			name:         "all empty",
			err:          APIError{},
			wantContains: "HTTP 0",
		},
		{
			name:         "code only",
			err:          APIError{ErrorCode: "SomeCode"},
			wantContains: "SomeCode: ",
		},
		{
			name:         "message only",
			err:          APIError{ErrorMessage: "some message"},
			wantContains: "some message",
		},
		{
			name:         "status only",
			err:          APIError{HTTPStatus: 503},
			wantContains: "HTTP 503",
		},
		{
			name:         "code and message",
			err:          APIError{ErrorCode: "BadRequest", ErrorMessage: "invalid"},
			wantContains: "BadRequest: invalid",
		},
		{
			name:         "code and status",
			err:          APIError{ErrorCode: "Err", HTTPStatus: 500},
			wantContains: "Err: ",
		},
		{
			name:         "message and status",
			err:          APIError{ErrorMessage: "fail", HTTPStatus: 500},
			wantContains: "fail",
		},
		{
			name:         "all populated",
			err:          APIError{ErrorCode: "Code", ErrorMessage: "Msg", HTTPStatus: 422},
			wantContains: "Code: Msg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got == "" {
				t.Error("Error() should never return empty string")
			}
			if !contains(got, tt.wantContains) {
				t.Errorf("Error() = %q, want to contain %q", got, tt.wantContains)
			}
		})
	}
}
