package apierror

import (
	"errors"
	"fmt"
	"strings"
)

// Category classifies an error by its nature.
type Category int

const (
	CategoryUnknown    Category = iota
	CategoryNetwork             // connection refused, DNS, timeout
	CategoryAuth                // 401, 403
	CategoryPlan                // plan feature/quota restriction
	CategoryValidation          // 400
	CategoryNotFound            // 404
	CategoryRateLimit           // 429
	CategoryServer              // 5xx
)

// APIError represents a structured error from the SOLAPI REST API.
type APIError struct {
	HTTPStatus   int
	ErrorCode    string
	ErrorMessage string
}

func (e *APIError) Error() string {
	if e.ErrorCode != "" {
		return fmt.Sprintf("%s: %s", e.ErrorCode, e.ErrorMessage)
	}
	if e.ErrorMessage != "" {
		return e.ErrorMessage
	}
	return fmt.Sprintf("HTTP %d", e.HTTPStatus)
}

// ClassifiedError wraps an error with a user-friendly category, message, and hint.
type ClassifiedError struct {
	Original error
	Category Category
	Message  string // Korean user message
	Hint     string // Korean action hint
}

func (e *ClassifiedError) Error() string {
	return e.Original.Error()
}

func (e *ClassifiedError) Unwrap() error {
	return e.Original
}

// Classify inspects err and returns a ClassifiedError with a user-friendly
// Korean message and actionable hint.
func Classify(err error) *ClassifiedError {
	if err == nil {
		return nil
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return classifyAPI(apiErr)
	}

	return classifyGeneric(err)
}

func classifyAPI(e *APIError) *ClassifiedError {
	ce := &ClassifiedError{Original: e}

	switch {
	case strings.EqualFold(e.ErrorCode, "PlanQuotaExceeded"):
		ce.Category = CategoryPlan
		ce.Message = "CRM 플랜 한도를 초과했습니다"
		ce.Hint = firstNonEmpty(e.ErrorMessage, "현재 플랜의 사용량 또는 한도를 확인하세요")

	case strings.EqualFold(e.ErrorCode, "PlanFeatureDisabled"):
		ce.Category = CategoryPlan
		ce.Message = "현재 CRM 플랜에서 사용할 수 없는 기능입니다"
		ce.Hint = firstNonEmpty(e.ErrorMessage, "필요한 기능이 포함된 플랜인지 확인하세요")

	case e.HTTPStatus == 401 || strings.EqualFold(e.ErrorCode, "Unauthorized"):
		ce.Category = CategoryAuth
		ce.Message = "인증에 실패했습니다"
		ce.Hint = "API 키를 확인하세요: solactl configure show"

	case e.HTTPStatus == 403 || strings.EqualFold(e.ErrorCode, "Forbidden"):
		ce.Category = CategoryAuth
		ce.Message = "접근 권한이 없습니다"
		ce.Hint = "API 키의 권한을 확인하세요"

	case e.HTTPStatus == 404 || strings.EqualFold(e.ErrorCode, "NotFound"):
		ce.Category = CategoryNotFound
		ce.Message = "리소스를 찾을 수 없습니다"
		ce.Hint = "ID를 확인하세요"

	case e.HTTPStatus == 400 || strings.EqualFold(e.ErrorCode, "BadRequest") || strings.EqualFold(e.ErrorCode, "ValidationError"):
		ce.Category = CategoryValidation
		ce.Message = "입력값이 올바르지 않습니다"
		if e.ErrorMessage != "" {
			ce.Hint = e.ErrorMessage
		}

	case e.HTTPStatus == 429 || strings.EqualFold(e.ErrorCode, "TooManyRequests"):
		ce.Category = CategoryRateLimit
		ce.Message = "요청 한도를 초과했습니다"
		ce.Hint = "잠시 후 다시 시도하세요"

	case e.HTTPStatus >= 500 || strings.EqualFold(e.ErrorCode, "InternalServerError"):
		ce.Category = CategoryServer
		ce.Message = "서버 오류가 발생했습니다"
		ce.Hint = "잠시 후 다시 시도하세요"

	default:
		ce.Category = CategoryUnknown
		ce.Message = e.ErrorMessage
		if ce.Message == "" {
			ce.Message = e.Error()
		}
	}

	return ce
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func classifyGeneric(err error) *ClassifiedError {
	ce := &ClassifiedError{Original: err}
	msg := err.Error()

	switch {
	case strings.Contains(msg, "connection refused"):
		ce.Category = CategoryNetwork
		ce.Message = "서버에 연결할 수 없습니다"
		ce.Hint = "네트워크 연결 및 서버 URL을 확인하세요"

	case strings.Contains(msg, "no such host"):
		ce.Category = CategoryNetwork
		ce.Message = "서버 주소를 찾을 수 없습니다"
		ce.Hint = "서버 URL을 확인하세요: solactl configure show"

	case strings.Contains(msg, "context deadline exceeded"):
		ce.Category = CategoryNetwork
		ce.Message = "요청 시간이 초과되었습니다"
		ce.Hint = "--timeout 옵션으로 타임아웃을 늘려보세요"

	case strings.Contains(msg, "context canceled"):
		ce.Category = CategoryUnknown
		ce.Message = "요청이 취소되었습니다"

	case strings.Contains(msg, "i/o timeout") || strings.Contains(msg, "deadline exceeded"):
		ce.Category = CategoryNetwork
		ce.Message = "네트워크 연결 시간이 초과되었습니다"
		ce.Hint = "네트워크 연결을 확인하세요"

	case strings.Contains(msg, "EOF") || strings.Contains(msg, "connection reset"):
		ce.Category = CategoryNetwork
		ce.Message = "서버와의 연결이 끊어졌습니다"
		ce.Hint = "잠시 후 다시 시도하세요"

	default:
		ce.Category = CategoryUnknown
		ce.Message = err.Error()
	}

	return ce
}
