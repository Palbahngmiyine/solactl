package validation

import (
	"fmt"
	"strings"

	"github.com/solapi/solactl/pkg/types"
)

// ValidateMessages validates all messages and returns collected errors.
// Returns nil if all messages are valid.
// This function may mutate msg.To (phone normalization) and msg.Type (auto-detection).
func ValidateMessages(msgs []types.Message, opts Options) ValidationErrors {
	var errs ValidationErrors

	for i := range msgs {
		// Common validation (phone normalization, customFields)
		errs = append(errs, validateCommon(&msgs[i], i, opts)...)

		// Auto-detect type if not set
		if msgs[i].Type == "" && opts.AutoTypeDetect {
			msgs[i].Type = DetectType(&msgs[i])
		}

		// Type-specific validation
		msgType := msgs[i].Type
		switch {
		case msgType == "SMS":
			errs = append(errs, validateSMS(&msgs[i], i, opts)...)
		case msgType == "LMS":
			errs = append(errs, validateLMS(&msgs[i], i, opts)...)
		case msgType == "MMS":
			errs = append(errs, validateMMS(&msgs[i], i, opts)...)
		case msgType == "ATA":
			errs = append(errs, validateATA(&msgs[i], i, opts)...)
		case strings.HasPrefix(msgType, "BMS"):
			errs = append(errs, validateBMS(&msgs[i], i, msgType, opts)...)
		case strings.HasPrefix(msgType, "RCS"):
			errs = append(errs, validateRCS(&msgs[i], i, msgType, opts)...)
		case msgType == "":
			// No type set and auto-detect disabled — skip type-specific validation
		default:
			errs = append(errs, ValidationError{
				Index: i, Field: "type", Code: "1010",
				Message: fmt.Sprintf("알 수 없는 메시지 타입입니다: %q", msgType),
			})
		}
	}

	// Duplicate recipient check
	if !opts.AllowDuplicates {
		errs = append(errs, checkDuplicateRecipients(msgs)...)
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}
