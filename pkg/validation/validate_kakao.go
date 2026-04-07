package validation

import (
	"fmt"

	"github.com/solapi/solactl/pkg/types"
)

// validateATA validates a Kakao AlimTalk (ATA) message.
func validateATA(msg *types.Message, idx int, _ Options) []ValidationError {
	var errs []ValidationError

	// from is optional for Kakao types
	// errs = append(errs, validateFrom(msg, idx, false)...)

	// text required
	if msg.Text == "" {
		errs = append(errs, ValidationError{
			Index: idx, Field: "text", Code: "1010",
			Message: "ATA 본문(--text)은 필수입니다",
		})
	} else {
		textLen := GetRealTextLength(msg.Text)
		if textLen > 1000 {
			errs = append(errs, ValidationError{
				Index: idx, Field: "text", Code: "1031",
				Message: fmt.Sprintf("ATA 본문은 1,000자 이하여야 합니다 (현재: %d자)", textLen),
			})
		}
	}

	// kakaoOptions required
	if msg.KakaoOptions == nil {
		errs = append(errs, ValidationError{
			Index: idx, Field: "kakaoOptions", Code: "1010",
			Message: "ATA는 kakaoOptions가 필수입니다",
		})
		return errs
	}

	ko := msg.KakaoOptions

	// pfId or senderKey required
	if ko.PfID == "" && ko.SenderKey == "" {
		errs = append(errs, ValidationError{
			Index: idx, Field: "kakaoOptions.pfId", Code: "1010",
			Message: "ATA는 pfId 또는 senderKey가 필수입니다 (--pf-id)",
		})
	}

	// templateId required
	if ko.TemplateID == "" {
		errs = append(errs, ValidationError{
			Index: idx, Field: "kakaoOptions.templateId", Code: "1010",
			Message: "ATA는 templateId가 필수입니다 (--template-id)",
		})
	}

	// buttons max 5
	if len(ko.Buttons) > 5 {
		errs = append(errs, ValidationError{
			Index: idx, Field: "kakaoOptions.buttons", Code: "1044",
			Message: fmt.Sprintf("ATA 버튼은 최대 5개입니다 (현재: %d개)", len(ko.Buttons)),
		})
	}

	return errs
}
