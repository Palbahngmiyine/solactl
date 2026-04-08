package validation

import (
	"fmt"

	"github.com/solapi/solactl/pkg/types"
)

// validateRCS validates an RCS message by sub-type.
func validateRCS(msg *types.Message, idx int, detectedType string, opts Options) []ValidationError {
	switch detectedType {
	case "RCS_SMS":
		return validateRCSSMS(msg, idx, opts)
	case "RCS_LMS":
		return validateRCSLMS(msg, idx, opts)
	case "RCS_MMS":
		return validateRCSMMS(msg, idx, opts)
	case "RCS_TPL":
		return validateRCSTPL(msg, idx, opts)
	default:
		return []ValidationError{{
			Index: idx, Field: "type", Code: "1010",
			Message: fmt.Sprintf("알 수 없는 RCS 서브타입입니다: %q", detectedType),
		}}
	}
}

func validateRCSSMS(msg *types.Message, idx int, _ Options) []ValidationError {
	var errs []ValidationError

	errs = append(errs, validateFrom(msg, idx, true)...)
	errs = append(errs, validateRCSBrandID(msg, idx)...)

	// text required
	if msg.Text == "" {
		errs = append(errs, ValidationError{
			Index: idx, Field: "text", Code: "1010",
			Message: "RCS_SMS 본문(--text)은 필수입니다",
		})
	} else {
		textLen := GetRealTextLength(msg.Text)
		if textLen > 100 {
			errs = append(errs, ValidationError{
				Index: idx, Field: "text", Code: "1031",
				Message: fmt.Sprintf("RCS_SMS 본문은 100자 이하여야 합니다 (현재: %d자)", textLen),
			})
		}
	}

	// imageId forbidden
	if msg.ImageID != "" {
		errs = append(errs, ValidationError{
			Index: idx, Field: "imageId", Code: "1010",
			Message: "imageId는 RCS_SMS에서 사용할 수 없습니다 (RCS_MMS를 사용하세요)",
		})
	}

	return errs
}

func validateRCSLMS(msg *types.Message, idx int, opts Options) []ValidationError {
	var errs []ValidationError

	errs = append(errs, validateFrom(msg, idx, true)...)
	errs = append(errs, validateRCSBrandID(msg, idx)...)

	// text required
	if msg.Text == "" {
		errs = append(errs, ValidationError{
			Index: idx, Field: "text", Code: "1010",
			Message: "RCS_LMS 본문(--text)은 필수입니다",
		})
	} else {
		textLen := GetRealTextLength(msg.Text)
		if textLen > 1300 {
			errs = append(errs, ValidationError{
				Index: idx, Field: "text", Code: "1031",
				Message: fmt.Sprintf("RCS_LMS 본문은 1,300자 이하여야 합니다 (현재: %d자)", textLen),
			})
		}
	}

	// subject max 30 chars
	if msg.Subject != "" {
		subjectLen := GetRealTextLength(msg.Subject)
		if subjectLen > 30 {
			if opts.Strict {
				errs = append(errs, ValidationError{
					Index: idx, Field: "subject", Code: "1014",
					Message: fmt.Sprintf("RCS_LMS 제목은 30자 이하여야 합니다 (현재: %d자)", subjectLen),
				})
			}
			// non-strict: auto-truncation on server
		}
	}

	// imageId forbidden
	if msg.ImageID != "" {
		errs = append(errs, ValidationError{
			Index: idx, Field: "imageId", Code: "1010",
			Message: "imageId는 RCS_LMS에서 사용할 수 없습니다 (RCS_MMS를 사용하세요)",
		})
	}

	return errs
}

func validateRCSMMS(msg *types.Message, idx int, opts Options) []ValidationError {
	var errs []ValidationError

	errs = append(errs, validateFrom(msg, idx, true)...)
	errs = append(errs, validateRCSBrandID(msg, idx)...)

	// text required
	if msg.Text == "" {
		errs = append(errs, ValidationError{
			Index: idx, Field: "text", Code: "1010",
			Message: "RCS_MMS 본문(--text)은 필수입니다",
		})
	} else {
		textLen := GetRealTextLength(msg.Text)
		if textLen > 1300 {
			errs = append(errs, ValidationError{
				Index: idx, Field: "text", Code: "1031",
				Message: fmt.Sprintf("RCS_MMS 본문은 1,300자 이하여야 합니다 (현재: %d자)", textLen),
			})
		}
	}

	// imageId required unless mmsType is set (template may provide the image)
	hasMmsType := msg.RCSOptions != nil && msg.RCSOptions.MmsType != ""
	if msg.ImageID == "" && !hasMmsType {
		errs = append(errs, ValidationError{
			Index: idx, Field: "imageId", Code: "1010",
			Message: "RCS_MMS는 이미지(--image) 또는 mmsType이 필요합니다",
		})
	}

	// subject max 30 chars
	if msg.Subject != "" {
		subjectLen := GetRealTextLength(msg.Subject)
		if subjectLen > 30 {
			if opts.Strict {
				errs = append(errs, ValidationError{
					Index: idx, Field: "subject", Code: "1014",
					Message: fmt.Sprintf("RCS_MMS 제목은 30자 이하여야 합니다 (현재: %d자)", subjectLen),
				})
			}
		}
	}

	return errs
}

func validateRCSTPL(msg *types.Message, idx int, _ Options) []ValidationError {
	var errs []ValidationError

	errs = append(errs, validateFrom(msg, idx, true)...)
	errs = append(errs, validateRCSBrandID(msg, idx)...)

	// templateId required (via RCSOptions)
	if msg.RCSOptions == nil || msg.RCSOptions.TemplateID == "" {
		errs = append(errs, ValidationError{
			Index: idx, Field: "rcsOptions.templateId", Code: "1010",
			Message: "RCS_TPL은 templateId가 필수입니다 (--template-id)",
		})
	}

	// text max 2600 chars
	if msg.Text != "" {
		textLen := GetRealTextLength(msg.Text)
		if textLen > 2600 {
			errs = append(errs, ValidationError{
				Index: idx, Field: "text", Code: "1031",
				Message: fmt.Sprintf("RCS_TPL 본문은 2,600자 이하여야 합니다 (현재: %d자)", textLen),
			})
		}
	}

	// subject max 60 chars
	if msg.Subject != "" {
		subjectLen := GetRealTextLength(msg.Subject)
		if subjectLen > 60 {
			errs = append(errs, ValidationError{
				Index: idx, Field: "subject", Code: "1014",
				Message: fmt.Sprintf("RCS_TPL 제목은 60자 이하여야 합니다 (현재: %d자)", subjectLen),
			})
		}
	}

	return errs
}

func validateRCSBrandID(msg *types.Message, idx int) []ValidationError {
	if msg.RCSOptions == nil || msg.RCSOptions.BrandID == "" {
		return []ValidationError{{
			Index: idx, Field: "rcsOptions.brandId", Code: "1010",
			Message: "RCS는 brandId가 필수입니다 (--brand-id)",
		}}
	}
	return nil
}
