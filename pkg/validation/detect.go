package validation

import (
	"strings"

	"github.com/solapi/solactl/pkg/types"
)

// DetectType determines the message type from message content
// following the PRD auto-type detection algorithm.
func DetectType(msg *types.Message) string {
	// 1. Kakao options present?
	if msg.KakaoOptions != nil {
		return detectKakaoType(msg)
	}

	// 2. RCS options present?
	if msg.RCSOptions != nil {
		return detectRCSType(msg)
	}

	// 3. Standard SMS/LMS/MMS detection
	return detectStandardType(msg)
}

func detectKakaoType(msg *types.Message) string {
	ko := msg.KakaoOptions

	// Check for BMS: templateId starts with KA01BP or bms field present
	isBMS := false
	if ko.TemplateID != "" && strings.HasPrefix(ko.TemplateID, "KA01BP") {
		isBMS = true
	}
	if ko.BMS != nil {
		isBMS = true
	}

	if isBMS {
		if ko.TemplateID != "" {
			// Template-based BMS: type from chatBubbleType
			return detectBMSTemplateType(ko)
		}
		// Free-form BMS
		return detectBMSFreeType(ko)
	}

	// ATA: templateCode or templateId present (non-KA01BP)
	if ko.TemplateID != "" {
		return "ATA"
	}

	// Fallback: if kakaoOptions present but no template, still ATA-like
	return "ATA"
}

func detectBMSTemplateType(ko *types.KakaoOptions) string {
	if ko.BMS != nil && ko.BMS.ChatBubbleType != "" {
		return "BMS_" + ko.BMS.ChatBubbleType
	}
	// Default to BMS_TEXT if no chatBubbleType
	return "BMS_TEXT"
}

func detectBMSFreeType(ko *types.KakaoOptions) string {
	if ko.BMS != nil && ko.BMS.ChatBubbleType != "" {
		return "BMS_" + ko.BMS.ChatBubbleType
	}
	return "BMS_TEXT"
}

func detectRCSType(msg *types.Message) string {
	ro := msg.RCSOptions

	// templateId present → RCS_TPL
	if ro.TemplateID != "" {
		return "RCS_TPL"
	}

	// imageId, mmsType, or additionalBody present → RCS_MMS
	if msg.ImageID != "" || ro.MmsType != "" {
		return "RCS_MMS"
	}

	// text > 100 chars or subject present → RCS_LMS
	if GetRealTextLength(msg.Text) > 100 || msg.Subject != "" {
		return "RCS_LMS"
	}

	return "RCS_SMS"
}

func detectStandardType(msg *types.Message) string {
	// imageId present → MMS
	if msg.ImageID != "" {
		return "MMS"
	}

	// text > 90 bytes or subject present → LMS
	if GetTextLength(msg.Text) > 90 || msg.Subject != "" {
		return "LMS"
	}

	return "SMS"
}
