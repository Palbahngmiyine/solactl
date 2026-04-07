package validation

import (
	"regexp"
	"strings"
)

var phoneDigitsOnly = regexp.MustCompile(`^[0-9]{5,25}$`)

// NormalizePhone strips all characters except digits and leading '+' from
// a phone number string. Formatting characters (-, space, parentheses),
// non-ASCII characters, and any other non-digit characters are removed.
func NormalizePhone(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i, r := range s {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+' && i == 0:
			// keep leading '+' only at position 0
			b.WriteRune(r)
		default:
			// drop all other characters (formatting, letters, symbols, non-ASCII)
		}
	}
	return b.String()
}

// ParsePhone normalizes a phone number and extracts an optional country code.
// Returns (normalized digits, country code string, error).
// Currently only supports Korean country code (+82). Other country code
// prefixes are passed through as-is (server handles validation).
func ParsePhone(s string) (number string, country string, err error) {
	cleaned := NormalizePhone(s)
	if cleaned == "" {
		return "", "", &PhoneError{Input: s, Reason: "수신번호가 비어 있습니다"}
	}

	// Parse country code from leading '+'
	if strings.HasPrefix(cleaned, "+") {
		rest := cleaned[1:]
		if len(rest) == 0 {
			return "", "", &PhoneError{Input: s, Reason: "국가 코드를 파싱할 수 없습니다"}
		}
		country, number = extractCountryCode(rest)
		if country == "" {
			// Non-Korean country code: pass through entire digits without
			// client-side normalization. Server handles country code validation.
			number = rest
			country = ""
		}
	} else {
		number = cleaned
		country = ""
	}

	// Korea-specific normalization: prepend 0 for numbers starting with 1.
	// Only applies when country code is explicitly "82" (from + prefix).
	if country == "82" && len(number) > 0 && number[0] == '1' {
		if !isKoreanSpecialNumber(number) {
			number = "0" + number
		}
	}

	// Validate final digit-only format
	if !phoneDigitsOnly.MatchString(number) {
		if len(number) < 5 {
			return "", "", &PhoneError{Input: s, Reason: "수신번호가 너무 짧습니다 (최소 5자리)"}
		}
		if len(number) > 25 {
			return "", "", &PhoneError{Input: s, Reason: "수신번호가 너무 깁니다 (최대 25자리)"}
		}
		return "", "", &PhoneError{Input: s, Reason: "수신번호는 숫자만 포함해야 합니다"}
	}

	return number, country, nil
}

// extractCountryCode extracts the country code from a digit string after '+'.
// Supports Korean country code "82" explicitly. For other prefixes,
// uses a 2-digit default (server validates the actual country code).
func extractCountryCode(s string) (country, rest string) {
	if len(s) < 2 {
		return "", ""
	}

	// Check for Korea (+82) explicitly
	if strings.HasPrefix(s, "82") {
		return "82", s[2:]
	}

	// For other country codes, pass the entire number through as-is.
	// The server handles country code validation for non-Korean numbers.
	// We return empty country to indicate no client-side normalization needed.
	return "", ""
}

// isKoreanSpecialNumber checks if a number starting with '1' is a
// special 8-digit Korean number (15xx, 16xx, 18xx) that should NOT
// have 0 prepended.
func isKoreanSpecialNumber(number string) bool {
	if len(number) != 8 {
		return false
	}
	prefix := number[:2]
	return prefix == "15" || prefix == "16" || prefix == "18"
}

// PhoneError represents a phone number validation error.
type PhoneError struct {
	Input  string
	Reason string
}

func (e *PhoneError) Error() string {
	return e.Reason + ": " + e.Input
}
