package validation

import (
	"fmt"
	"strings"
)

// ValidationError represents a single validation error for a message.
type ValidationError struct {
	Index   int    // message index in the batch (0-based)
	Field   string // field path (e.g., "to", "kakaoOptions.pfId")
	Code    string // SOLAPI error code (e.g., "1010", "1031")
	Message string // Korean error message
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []ValidationError

// Error implements the error interface.
func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "검증 오류 %d건", len(ve))
	for _, e := range ve {
		fmt.Fprintf(&b, "\n[%d] %s (%s): %s", e.Index, e.Field, e.Code, e.Message)
	}
	return b.String()
}

// HasErrors returns true if there are any validation errors.
func (ve ValidationErrors) HasErrors() bool {
	return len(ve) > 0
}

// Options controls validation behavior.
type Options struct {
	Strict          bool // strict mode: enforce template matching, subject requirements
	AllowDuplicates bool // allow duplicate recipients
	AutoTypeDetect  bool // auto-detect message type when Type is empty
}
