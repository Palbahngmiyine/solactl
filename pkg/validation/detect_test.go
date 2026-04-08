package validation

import (
	"strings"
	"testing"

	"github.com/solapi/solactl/pkg/types"
)

func TestDetectType(t *testing.T) {
	tests := []struct {
		name string
		msg  types.Message
		want string
	}{
		// Standard types
		{
			name: "sms_short_text",
			msg:  types.Message{Text: "hello"},
			want: "SMS",
		},
		{
			name: "sms_exactly_90bytes",
			msg:  types.Message{Text: strings.Repeat("a", 90)},
			want: "SMS",
		},
		{
			name: "lms_91bytes",
			msg:  types.Message{Text: strings.Repeat("a", 91)},
			want: "LMS",
		},
		{
			name: "lms_korean_46chars_92bytes",
			msg:  types.Message{Text: strings.Repeat("가", 46)},
			want: "LMS",
		},
		{
			name: "lms_with_subject",
			msg:  types.Message{Text: "short", Subject: "제목"},
			want: "LMS",
		},
		{
			name: "mms_with_imageId",
			msg:  types.Message{Text: "hello", ImageID: "img-123"},
			want: "MMS",
		},
		{
			name: "mms_imageId_overrides_length",
			msg:  types.Message{Text: "short", ImageID: "img-123"},
			want: "MMS",
		},
		{
			name: "sms_empty_text",
			msg:  types.Message{Text: ""},
			want: "SMS",
		},

		// Kakao ATA
		{
			name: "ata_with_templateId",
			msg: types.Message{
				Text:         "알림톡 내용",
				KakaoOptions: &types.KakaoOptions{TemplateID: "TPL_001"},
			},
			want: "ATA",
		},
		{
			name: "ata_with_pfId_and_templateId",
			msg: types.Message{
				Text:         "알림톡",
				KakaoOptions: &types.KakaoOptions{PfID: "KA01PF", TemplateID: "TPL_002"},
			},
			want: "ATA",
		},
		{
			name: "ata_kakaoOptions_no_template",
			msg: types.Message{
				Text:         "알림톡",
				KakaoOptions: &types.KakaoOptions{PfID: "KA01PF"},
			},
			want: "ATA",
		},

		// BMS Template types
		{
			name: "bms_text_template",
			msg: types.Message{
				KakaoOptions: &types.KakaoOptions{
					TemplateID: "KA01BP_TEXT_001",
					BMS:        &types.KakaoBMSOptions{ChatBubbleType: "TEXT", Targeting: "I"},
				},
			},
			want: "BMS_TEXT",
		},
		{
			name: "bms_image_template",
			msg: types.Message{
				KakaoOptions: &types.KakaoOptions{
					TemplateID: "KA01BP_IMG_001",
					BMS:        &types.KakaoBMSOptions{ChatBubbleType: "IMAGE", Targeting: "M"},
				},
			},
			want: "BMS_IMAGE",
		},
		{
			name: "bms_wide_template",
			msg: types.Message{
				KakaoOptions: &types.KakaoOptions{
					TemplateID: "KA01BP_WIDE_001",
					BMS:        &types.KakaoBMSOptions{ChatBubbleType: "WIDE", Targeting: "N"},
				},
			},
			want: "BMS_WIDE",
		},
		{
			name: "bms_wide_item_list_template",
			msg: types.Message{
				KakaoOptions: &types.KakaoOptions{
					TemplateID: "KA01BP_WIL_001",
					BMS:        &types.KakaoBMSOptions{ChatBubbleType: "WIDE_ITEM_LIST"},
				},
			},
			want: "BMS_WIDE_ITEM_LIST",
		},
		{
			name: "bms_commerce_template",
			msg: types.Message{
				KakaoOptions: &types.KakaoOptions{
					TemplateID: "KA01BP_COM_001",
					BMS:        &types.KakaoBMSOptions{ChatBubbleType: "COMMERCE"},
				},
			},
			want: "BMS_COMMERCE",
		},
		{
			name: "bms_carousel_feed_template",
			msg: types.Message{
				KakaoOptions: &types.KakaoOptions{
					TemplateID: "KA01BP_CF_001",
					BMS:        &types.KakaoBMSOptions{ChatBubbleType: "CAROUSEL_FEED"},
				},
			},
			want: "BMS_CAROUSEL_FEED",
		},
		{
			name: "bms_carousel_commerce_template",
			msg: types.Message{
				KakaoOptions: &types.KakaoOptions{
					TemplateID: "KA01BP_CC_001",
					BMS:        &types.KakaoBMSOptions{ChatBubbleType: "CAROUSEL_COMMERCE"},
				},
			},
			want: "BMS_CAROUSEL_COMMERCE",
		},
		{
			name: "bms_premium_video_template",
			msg: types.Message{
				KakaoOptions: &types.KakaoOptions{
					TemplateID: "KA01BP_PV_001",
					BMS:        &types.KakaoBMSOptions{ChatBubbleType: "PREMIUM_VIDEO"},
				},
			},
			want: "BMS_PREMIUM_VIDEO",
		},

		// BMS Free (no templateId)
		{
			name: "bms_free_text",
			msg: types.Message{
				KakaoOptions: &types.KakaoOptions{
					PfID: "KA01PF",
					BMS:  &types.KakaoBMSOptions{ChatBubbleType: "TEXT", Targeting: "I"},
				},
			},
			want: "BMS_TEXT",
		},
		{
			name: "bms_free_image",
			msg: types.Message{
				KakaoOptions: &types.KakaoOptions{
					PfID: "KA01PF",
					BMS:  &types.KakaoBMSOptions{ChatBubbleType: "IMAGE", Targeting: "M"},
				},
			},
			want: "BMS_IMAGE",
		},
		{
			name: "bms_free_wide",
			msg: types.Message{
				KakaoOptions: &types.KakaoOptions{
					PfID: "KA01PF",
					BMS:  &types.KakaoBMSOptions{ChatBubbleType: "WIDE", Targeting: "N"},
				},
			},
			want: "BMS_WIDE",
		},
		{
			name: "bms_free_no_bubbletype_defaults_text",
			msg: types.Message{
				KakaoOptions: &types.KakaoOptions{
					PfID: "KA01PF",
					BMS:  &types.KakaoBMSOptions{Targeting: "I"},
				},
			},
			want: "BMS_TEXT",
		},
		{
			name: "bms_KA01BP_prefix_without_bms_field",
			msg: types.Message{
				KakaoOptions: &types.KakaoOptions{
					TemplateID: "KA01BP_TEXT_001",
				},
			},
			want: "BMS_TEXT",
		},

		// RCS types
		{
			name: "rcs_sms_short",
			msg: types.Message{
				Text:       "짧은 메시지",
				RCSOptions: &types.RCSOptions{BrandID: "brand-1"},
			},
			want: "RCS_SMS",
		},
		{
			name: "rcs_sms_exactly_100chars",
			msg: types.Message{
				Text:       strings.Repeat("가", 100),
				RCSOptions: &types.RCSOptions{BrandID: "brand-1"},
			},
			want: "RCS_SMS",
		},
		{
			name: "rcs_lms_101chars",
			msg: types.Message{
				Text:       strings.Repeat("가", 101),
				RCSOptions: &types.RCSOptions{BrandID: "brand-1"},
			},
			want: "RCS_LMS",
		},
		{
			name: "rcs_lms_with_subject",
			msg: types.Message{
				Text:       "짧은",
				Subject:    "제목",
				RCSOptions: &types.RCSOptions{BrandID: "brand-1"},
			},
			want: "RCS_LMS",
		},
		{
			name: "rcs_mms_with_imageId",
			msg: types.Message{
				Text:       "짧은",
				ImageID:    "img-1",
				RCSOptions: &types.RCSOptions{BrandID: "brand-1"},
			},
			want: "RCS_MMS",
		},
		{
			name: "rcs_mms_with_mmsType",
			msg: types.Message{
				Text:       "짧은",
				RCSOptions: &types.RCSOptions{BrandID: "brand-1", MmsType: "M3"},
			},
			want: "RCS_MMS",
		},
		{
			name: "rcs_tpl_with_templateId",
			msg: types.Message{
				RCSOptions: &types.RCSOptions{BrandID: "brand-1", TemplateID: "tpl-1"},
			},
			want: "RCS_TPL",
		},
		{
			name: "rcs_tpl_overrides_mmsType",
			msg: types.Message{
				RCSOptions: &types.RCSOptions{BrandID: "brand-1", TemplateID: "tpl-1", MmsType: "M3"},
			},
			want: "RCS_TPL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			got := DetectType(&tt.msg)
			if got != tt.want {
				t.Errorf("DetectType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectType_PriorityOrder(t *testing.T) {
	// Kakao options take priority over RCS and standard
	t.Run("kakao_over_rcs", func(t *testing.T) {
		t.Cleanup(func() {})
		msg := types.Message{
			Text:         "text",
			KakaoOptions: &types.KakaoOptions{TemplateID: "TPL_001"},
			RCSOptions:   &types.RCSOptions{BrandID: "brand-1"},
		}
		got := DetectType(&msg)
		if got != "ATA" {
			t.Errorf("DetectType() = %q, want ATA (kakao priority)", got)
		}
	})

	// Kakao options take priority over imageId (MMS)
	t.Run("kakao_over_mms", func(t *testing.T) {
		t.Cleanup(func() {})
		msg := types.Message{
			Text:         "text",
			ImageID:      "img-1",
			KakaoOptions: &types.KakaoOptions{TemplateID: "TPL_001"},
		}
		got := DetectType(&msg)
		if got != "ATA" {
			t.Errorf("DetectType() = %q, want ATA (kakao priority over imageId)", got)
		}
	})

	// RCS takes priority over standard MMS
	t.Run("rcs_over_mms", func(t *testing.T) {
		t.Cleanup(func() {})
		msg := types.Message{
			Text:       "text",
			ImageID:    "img-1",
			RCSOptions: &types.RCSOptions{BrandID: "brand-1"},
		}
		got := DetectType(&msg)
		if got != "RCS_MMS" {
			t.Errorf("DetectType() = %q, want RCS_MMS (rcs priority)", got)
		}
	})
}
