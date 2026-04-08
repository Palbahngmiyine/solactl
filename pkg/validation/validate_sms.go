package validation

import (
	"fmt"

	"github.com/solapi/solactl/pkg/types"
)

// validateSMS validates an SMS message.
func validateSMS(msg *types.Message, idx int, opts Options) []ValidationError {
	var errs []ValidationError

	errs = append(errs, validateFrom(msg, idx, true)...)

	// text required
	if msg.Text == "" {
		errs = append(errs, ValidationError{
			Index: idx, Field: "text", Code: "1010",
			Message: "SMS 본문(--text)은 필수입니다",
		})
	} else {
		textLen := GetTextLength(msg.Text)
		if textLen > 90 {
			errs = append(errs, ValidationError{
				Index: idx, Field: "text", Code: "1031",
				Message: fmt.Sprintf("SMS 본문은 90바이트 이하여야 합니다 (현재: %d바이트)", textLen),
			})
		}
	}

	// subject forbidden
	if msg.Subject != "" {
		errs = append(errs, ValidationError{
			Index: idx, Field: "subject", Code: "1010",
			Message: "subject 필드는 SMS 타입에서 사용할 수 없습니다 (LMS로 전환하세요)",
		})
	}

	// imageId forbidden
	if msg.ImageID != "" {
		errs = append(errs, ValidationError{
			Index: idx, Field: "imageId", Code: "1010",
			Message: "imageId 필드는 SMS 타입에서 사용할 수 없습니다 (MMS로 전환하세요)",
		})
	}

	return errs
}

// validateLMS validates an LMS message.
func validateLMS(msg *types.Message, idx int, opts Options) []ValidationError {
	var errs []ValidationError

	errs = append(errs, validateFrom(msg, idx, true)...)

	// text required
	if msg.Text == "" {
		errs = append(errs, ValidationError{
			Index: idx, Field: "text", Code: "1010",
			Message: "LMS 본문(--text)은 필수입니다",
		})
	} else {
		textLen := GetTextLength(msg.Text)
		if textLen > 2000 {
			errs = append(errs, ValidationError{
				Index: idx, Field: "text", Code: "1031",
				Message: fmt.Sprintf("LMS 본문은 2,000바이트 이하여야 합니다 (현재: %d바이트)", textLen),
			})
		}
	}

	// subject validation
	if msg.Subject != "" {
		subjectLen := GetTextLength(msg.Subject)
		if opts.Strict && subjectLen > 40 {
			errs = append(errs, ValidationError{
				Index: idx, Field: "subject", Code: "1014",
				Message: fmt.Sprintf("제목은 40바이트 이하여야 합니다 (현재: %d바이트)", subjectLen),
			})
		}
		// non-strict: auto-truncation is done server-side, no error
	} else if opts.Strict {
		errs = append(errs, ValidationError{
			Index: idx, Field: "subject", Code: "1010",
			Message: "strict 모드에서 LMS 제목(--subject)은 필수입니다",
		})
	}

	// imageId forbidden
	if msg.ImageID != "" {
		errs = append(errs, ValidationError{
			Index: idx, Field: "imageId", Code: "1010",
			Message: "imageId 필드는 LMS 타입에서 사용할 수 없습니다 (MMS로 전환하세요)",
		})
	}

	return errs
}

// validateMMS validates an MMS message.
func validateMMS(msg *types.Message, idx int, opts Options) []ValidationError {
	var errs []ValidationError

	errs = append(errs, validateFrom(msg, idx, true)...)

	// text required
	if msg.Text == "" {
		errs = append(errs, ValidationError{
			Index: idx, Field: "text", Code: "1010",
			Message: "MMS 본문(--text)은 필수입니다",
		})
	} else {
		textLen := GetTextLength(msg.Text)
		if textLen > 2000 {
			errs = append(errs, ValidationError{
				Index: idx, Field: "text", Code: "1031",
				Message: fmt.Sprintf("MMS 본문은 2,000바이트 이하여야 합니다 (현재: %d바이트)", textLen),
			})
		}
	}

	// imageId required
	if msg.ImageID == "" {
		errs = append(errs, ValidationError{
			Index: idx, Field: "imageId", Code: "1010",
			Message: "MMS는 이미지(--image)가 필수입니다",
		})
	}

	// subject validation (same as LMS)
	if msg.Subject != "" {
		subjectLen := GetTextLength(msg.Subject)
		if opts.Strict && subjectLen > 40 {
			errs = append(errs, ValidationError{
				Index: idx, Field: "subject", Code: "1014",
				Message: fmt.Sprintf("제목은 40바이트 이하여야 합니다 (현재: %d바이트)", subjectLen),
			})
		}
	} else if opts.Strict {
		errs = append(errs, ValidationError{
			Index: idx, Field: "subject", Code: "1010",
			Message: "strict 모드에서 MMS 제목(--subject)은 필수입니다",
		})
	}

	return errs
}
