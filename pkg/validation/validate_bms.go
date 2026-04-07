package validation

import (
	"fmt"
	"strings"

	"github.com/solapi/solactl/pkg/types"
)

// validateBMS validates a BMS (Brand Message Service) message.
// detectedType should be one of: BMS_TEXT, BMS_IMAGE, BMS_WIDE,
// BMS_WIDE_ITEM_LIST, BMS_COMMERCE, BMS_CAROUSEL_FEED,
// BMS_CAROUSEL_COMMERCE, BMS_PREMIUM_VIDEO.
func validateBMS(msg *types.Message, idx int, detectedType string, _ Options) []ValidationError {
	var errs []ValidationError

	// kakaoOptions required
	if msg.KakaoOptions == nil {
		errs = append(errs, ValidationError{
			Index: idx, Field: "kakaoOptions", Code: "1010",
			Message: "BMS는 kakaoOptions가 필수입니다",
		})
		return errs
	}

	ko := msg.KakaoOptions

	// pfId or senderKey required
	if ko.PfID == "" && ko.SenderKey == "" {
		errs = append(errs, ValidationError{
			Index: idx, Field: "kakaoOptions.pfId", Code: "1010",
			Message: "BMS는 pfId 또는 senderKey가 필수입니다 (--pf-id)",
		})
	}

	// targeting required
	if ko.BMS == nil || ko.BMS.Targeting == "" {
		errs = append(errs, ValidationError{
			Index: idx, Field: "kakaoOptions.bms.targeting", Code: "1083",
			Message: "BMS는 targeting이 필수입니다 (--targeting: I, M, N)",
		})
	} else {
		t := ko.BMS.Targeting
		if t != "I" && t != "M" && t != "N" {
			errs = append(errs, ValidationError{
				Index: idx, Field: "kakaoOptions.bms.targeting", Code: "1083",
				Message: fmt.Sprintf("targeting 값이 올바르지 않습니다: %q (허용: I, M, N)", t),
			})
		}
	}

	// Template-based BMS: text/imageId are provided by the template, so
	// content validation is skipped — only common fields are validated.
	isTemplate := ko.TemplateID != ""
	if isTemplate {
		return errs
	}

	// Free-form BMS: type-specific content validation
	switch detectedType {
	case "BMS_TEXT":
		errs = append(errs, validateBMSText(msg, idx)...)
	case "BMS_IMAGE":
		errs = append(errs, validateBMSImage(msg, idx)...)
	case "BMS_WIDE":
		errs = append(errs, validateBMSWide(msg, idx)...)
	case "BMS_WIDE_ITEM_LIST":
		errs = append(errs, validateBMSWideItemList(msg, idx)...)
	case "BMS_COMMERCE":
		errs = append(errs, validateBMSCommerce(msg, idx)...)
	case "BMS_CAROUSEL_FEED":
		errs = append(errs, validateBMSCarousel(msg, idx, "CAROUSEL_FEED")...)
	case "BMS_CAROUSEL_COMMERCE":
		errs = append(errs, validateBMSCarousel(msg, idx, "CAROUSEL_COMMERCE")...)
	case "BMS_PREMIUM_VIDEO":
		errs = append(errs, validateBMSPremiumVideo(msg, idx)...)
	}

	return errs
}

func validateBMSText(msg *types.Message, idx int) []ValidationError {
	var errs []ValidationError

	if msg.Text == "" {
		errs = append(errs, ValidationError{
			Index: idx, Field: "text", Code: "1010",
			Message: "BMS_TEXT는 본문(content)이 필수입니다",
		})
	} else {
		textLen := GetJSStringLength(msg.Text)
		if textLen > 1300 {
			errs = append(errs, ValidationError{
				Index: idx, Field: "text", Code: "1031",
				Message: fmt.Sprintf("BMS_TEXT 본문은 1,300자 이하여야 합니다 (현재: %d자)", textLen),
			})
		}
		if countNewlines(msg.Text) > 99 {
			errs = append(errs, ValidationError{
				Index: idx, Field: "text", Code: "1031",
				Message: "BMS_TEXT 본문의 줄바꿈은 99개 이하여야 합니다",
			})
		}
	}

	errs = append(errs, validateBMSButtons(msg, idx, 5, 14)...)
	return errs
}

func validateBMSImage(msg *types.Message, idx int) []ValidationError {
	var errs []ValidationError

	if msg.Text == "" {
		errs = append(errs, ValidationError{
			Index: idx, Field: "text", Code: "1010",
			Message: "BMS_IMAGE는 본문(content)이 필수입니다",
		})
	} else {
		textLen := GetJSStringLength(msg.Text)
		if textLen > 1300 {
			errs = append(errs, ValidationError{
				Index: idx, Field: "text", Code: "1031",
				Message: fmt.Sprintf("BMS_IMAGE 본문은 1,300자 이하여야 합니다 (현재: %d자)", textLen),
			})
		}
	}

	if msg.ImageID == "" && (msg.KakaoOptions == nil || msg.KakaoOptions.ImageID == "") {
		errs = append(errs, ValidationError{
			Index: idx, Field: "imageId", Code: "1010",
			Message: "BMS_IMAGE는 이미지가 필수입니다",
		})
	}

	errs = append(errs, validateBMSButtons(msg, idx, 5, 14)...)
	return errs
}

func validateBMSWide(msg *types.Message, idx int) []ValidationError {
	var errs []ValidationError

	if msg.Text == "" {
		errs = append(errs, ValidationError{
			Index: idx, Field: "text", Code: "1010",
			Message: "BMS_WIDE는 본문(content)이 필수입니다",
		})
	} else {
		textLen := GetJSStringLength(msg.Text)
		if textLen > 76 {
			errs = append(errs, ValidationError{
				Index: idx, Field: "text", Code: "1031",
				Message: fmt.Sprintf("BMS_WIDE 본문은 76자 이하여야 합니다 (현재: %d자)", textLen),
			})
		}
		if countNewlines(msg.Text) > 1 {
			errs = append(errs, ValidationError{
				Index: idx, Field: "text", Code: "1031",
				Message: "BMS_WIDE 본문의 줄바꿈은 1개 이하여야 합니다",
			})
		}
	}

	if msg.ImageID == "" && (msg.KakaoOptions == nil || msg.KakaoOptions.ImageID == "") {
		errs = append(errs, ValidationError{
			Index: idx, Field: "imageId", Code: "1010",
			Message: "BMS_WIDE는 이미지가 필수입니다",
		})
	}

	errs = append(errs, validateBMSButtons(msg, idx, 2, 8)...)
	return errs
}

func validateBMSWideItemList(_ *types.Message, _ int) []ValidationError {
	// WIDE_ITEM_LIST requires complex structured fields (header, mainWideItem, subWideItemList)
	// which are not directly exposed as CLI flags. Minimal validation here.
	return nil
}

func validateBMSCommerce(msg *types.Message, idx int) []ValidationError {
	var errs []ValidationError

	if msg.ImageID == "" && (msg.KakaoOptions == nil || msg.KakaoOptions.ImageID == "") {
		errs = append(errs, ValidationError{
			Index: idx, Field: "imageId", Code: "1010",
			Message: "BMS_COMMERCE는 이미지가 필수입니다",
		})
	}

	// buttons 1-2, only WL/AL
	if msg.KakaoOptions != nil {
		nButtons := len(msg.KakaoOptions.Buttons)
		if nButtons < 1 {
			errs = append(errs, ValidationError{
				Index: idx, Field: "kakaoOptions.buttons", Code: "1010",
				Message: "BMS_COMMERCE는 버튼이 최소 1개 필수입니다",
			})
		}
		if nButtons > 2 {
			errs = append(errs, ValidationError{
				Index: idx, Field: "kakaoOptions.buttons", Code: "1044",
				Message: fmt.Sprintf("BMS_COMMERCE 버튼은 최대 2개입니다 (현재: %d개)", nButtons),
			})
		}
		for i, btn := range msg.KakaoOptions.Buttons {
			if btn.ButtonType != "WL" && btn.ButtonType != "AL" && btn.ButtonType != "" {
				errs = append(errs, ValidationError{
					Index: idx, Field: fmt.Sprintf("kakaoOptions.buttons[%d].buttonType", i), Code: "1010",
					Message: fmt.Sprintf("BMS_COMMERCE 버튼은 WL 또는 AL만 허용됩니다 (현재: %q)", btn.ButtonType),
				})
			}
		}
	}

	return errs
}

func validateBMSCarousel(_ *types.Message, _ int, _ string) []ValidationError {
	// CAROUSEL types require complex structured carousel fields
	// which are not directly exposed as CLI flags. Minimal validation.
	return nil
}

func validateBMSPremiumVideo(msg *types.Message, idx int) []ValidationError {
	var errs []ValidationError

	if msg.Text != "" {
		textLen := GetJSStringLength(msg.Text)
		if textLen > 76 {
			errs = append(errs, ValidationError{
				Index: idx, Field: "text", Code: "1031",
				Message: fmt.Sprintf("BMS_PREMIUM_VIDEO 본문은 76자 이하여야 합니다 (현재: %d자)", textLen),
			})
		}
	}

	// buttons max 1
	if msg.KakaoOptions != nil && len(msg.KakaoOptions.Buttons) > 1 {
		errs = append(errs, ValidationError{
			Index: idx, Field: "kakaoOptions.buttons", Code: "1044",
			Message: fmt.Sprintf("BMS_PREMIUM_VIDEO 버튼은 최대 1개입니다 (현재: %d개)", len(msg.KakaoOptions.Buttons)),
		})
	}

	return errs
}

// validateBMSButtons validates button count and name length for BMS types.
func validateBMSButtons(msg *types.Message, idx int, maxButtons int, maxNameLen int) []ValidationError {
	if msg.KakaoOptions == nil {
		return nil
	}
	var errs []ValidationError

	nButtons := len(msg.KakaoOptions.Buttons)
	if nButtons > maxButtons {
		errs = append(errs, ValidationError{
			Index: idx, Field: "kakaoOptions.buttons", Code: "1044",
			Message: fmt.Sprintf("버튼은 최대 %d개입니다 (현재: %d개)", maxButtons, nButtons),
		})
	}

	for i, btn := range msg.KakaoOptions.Buttons {
		nameLen := GetJSStringLength(btn.ButtonName)
		if nameLen > maxNameLen {
			errs = append(errs, ValidationError{
				Index: idx, Field: fmt.Sprintf("kakaoOptions.buttons[%d].buttonName", i), Code: "1010",
				Message: fmt.Sprintf("버튼명은 최대 %d자입니다 (현재: %d자)", maxNameLen, nameLen),
			})
		}
	}

	return errs
}

// countNewlines counts the number of newline characters in a string.
func countNewlines(s string) int {
	return strings.Count(s, "\n")
}
