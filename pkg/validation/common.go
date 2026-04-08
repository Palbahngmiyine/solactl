package validation

import (
	"strconv"

	"github.com/solapi/solactl/pkg/types"
)

// validateCommon performs common validation shared across all message types.
func validateCommon(msg *types.Message, idx int, _ Options) []ValidationError {
	var errs []ValidationError

	// Validate phone number
	if msg.To == "" {
		errs = append(errs, ValidationError{
			Index: idx, Field: "to", Code: "1010",
			Message: "수신번호(--to)가 비어 있습니다",
		})
	} else {
		number, country, err := ParsePhone(msg.To)
		if err != nil {
			errs = append(errs, ValidationError{
				Index: idx, Field: "to", Code: "1010",
				Message: err.Error(),
			})
		} else {
			msg.To = number // normalize in place
			if country != "" {
				msg.Country = country
			}
		}
	}

	// Validate customFields
	errs = append(errs, validateCustomFields(msg.CustomFields, idx)...)

	return errs
}

// validateFrom checks the from field requirement based on message type.
func validateFrom(msg *types.Message, idx int, required bool) []ValidationError {
	if required && msg.From == "" {
		return []ValidationError{{
			Index: idx, Field: "from", Code: "1010",
			Message: "발신번호(--from)는 필수입니다",
		}}
	}
	return nil
}

// validateCustomFields checks the custom fields constraints.
func validateCustomFields(fields map[string]string, idx int) []ValidationError {
	if fields == nil {
		return nil
	}
	var errs []ValidationError

	if len(fields) > 10 {
		errs = append(errs, ValidationError{
			Index: idx, Field: "customFields", Code: "1010",
			Message: "customFields는 최대 10개까지 허용됩니다",
		})
	}

	for key, value := range fields {
		keyLen := GetRealTextLength(key)
		if keyLen > 30 {
			errs = append(errs, ValidationError{
				Index: idx, Field: "customFields." + key, Code: "1010",
				Message: "customFields 키는 최대 30자까지 허용됩니다",
			})
		}
		valueLen := GetRealTextLength(value)
		if valueLen > 1000 {
			errs = append(errs, ValidationError{
				Index: idx, Field: "customFields." + key, Code: "1010",
				Message: "customFields 값은 최대 1,000자까지 허용됩니다",
			})
		}
	}

	return errs
}

// checkDuplicateRecipients finds duplicate recipient phone numbers.
func checkDuplicateRecipients(msgs []types.Message) []ValidationError {
	seen := make(map[string]int, len(msgs))
	var errs []ValidationError

	for i, msg := range msgs {
		if msg.To == "" {
			continue
		}
		if firstIdx, ok := seen[msg.To]; ok {
			errs = append(errs, ValidationError{
				Index: i, Field: "to", Code: "1026",
				Message: "중복 수신번호입니다 (메시지 #" + strconv.Itoa(firstIdx+1) + "과 동일)",
			})
		} else {
			seen[msg.To] = i
		}
	}

	return errs
}

