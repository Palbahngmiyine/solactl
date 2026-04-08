package validation

import (
	"fmt"
	"strings"
	"testing"

	"github.com/solapi/solactl/pkg/types"
)

// validBMSKakaoOptions returns KakaoOptions for template-based BMS (skips content validation).
func validBMSKakaoOptions(bubbleType string) *types.KakaoOptions {
	return &types.KakaoOptions{
		PfID:       "KA01PF_001",
		TemplateID: "KA01BP_001",
		BMS:        &types.KakaoBMSOptions{ChatBubbleType: bubbleType, Targeting: "I"},
	}
}

// freeBMSKakaoOptions returns KakaoOptions for free-form BMS (content validation applied).
func freeBMSKakaoOptions(bubbleType string) *types.KakaoOptions {
	return &types.KakaoOptions{
		PfID: "KA01PF_001",
		BMS:  &types.KakaoBMSOptions{ChatBubbleType: bubbleType, Targeting: "I"},
	}
}

func TestValidateBMS_CommonFields(t *testing.T) {
	tests := []struct {
		name      string
		msg       types.Message
		wantField string
	}{
		{
			name:      "kakaoOptions_nil",
			msg:       types.Message{To: "01012345678"},
			wantField: "kakaoOptions",
		},
		{
			name: "pfId_missing",
			msg: types.Message{
				To: "01012345678",
				KakaoOptions: &types.KakaoOptions{
					TemplateID: "KA01BP_001",
					BMS:        &types.KakaoBMSOptions{Targeting: "I"},
				},
			},
			wantField: "kakaoOptions.pfId",
		},
		{
			name: "targeting_missing",
			msg: types.Message{
				To:           "01012345678",
				KakaoOptions: &types.KakaoOptions{PfID: "KA01PF_001", TemplateID: "KA01BP_001"},
			},
			wantField: "kakaoOptions.bms.targeting",
		},
		{
			name: "targeting_invalid_value",
			msg: types.Message{
				To: "01012345678",
				KakaoOptions: &types.KakaoOptions{
					PfID: "KA01PF_001", TemplateID: "KA01BP_001",
					BMS: &types.KakaoBMSOptions{Targeting: "X"},
				},
			},
			wantField: "kakaoOptions.bms.targeting",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			errs := validateBMS(&tt.msg, 0, "BMS_TEXT", Options{})
			found := false
			for _, e := range errs {
				if e.Field == tt.wantField {
					found = true
				}
			}
			if !found {
				t.Errorf("expected error on field %q, got: %v", tt.wantField, errs)
			}
		})
	}
}

func TestValidateBMS_AllBubbleTypes(t *testing.T) {
	// Test that all 8 bubble types are dispatched without panic
	bubbleTypes := []string{
		"BMS_TEXT", "BMS_IMAGE", "BMS_WIDE",
		"BMS_WIDE_ITEM_LIST", "BMS_COMMERCE",
		"BMS_CAROUSEL_FEED", "BMS_CAROUSEL_COMMERCE",
		"BMS_PREMIUM_VIDEO",
	}

	for _, bt := range bubbleTypes {
		t.Run(bt, func(t *testing.T) {
			t.Cleanup(func() {})
			shortType := strings.TrimPrefix(bt, "BMS_")
			ko := validBMSKakaoOptions(shortType)
			msg := types.Message{
				To:           "01012345678",
				Text:         "content",
				ImageID:      "img-1",
				KakaoOptions: ko,
			}
			// Should not panic
			errs := validateBMS(&msg, 0, bt, Options{})
			_ = errs
		})
	}
}

func TestValidateBMS_TEXT(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantN   int
	}{
		{name: "valid", text: "hello", wantN: 0},
		{name: "1300chars_ok", text: strings.Repeat("가", 1300), wantN: 0},
		{name: "1301chars_fail", text: strings.Repeat("가", 1301), wantN: 1},
		{name: "empty_text_fail", text: "", wantN: 1},
		{name: "99newlines_ok", text: strings.Repeat("\n", 99) + "a", wantN: 0},
		{name: "100newlines_fail", text: strings.Repeat("\n", 100) + "a", wantN: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			msg := types.Message{To: "01012345678", Text: tt.text, KakaoOptions: freeBMSKakaoOptions("TEXT")}
			errs := validateBMS(&msg, 0, "BMS_TEXT", Options{})
			// Filter to text-related errors only
			textErrs := 0
			for _, e := range errs {
				if e.Field == "text" {
					textErrs++
				}
			}
			if textErrs != tt.wantN {
				t.Errorf("text errors = %d, want %d (all errs: %v)", textErrs, tt.wantN, errs)
			}
		})
	}
}

func TestValidateBMS_IMAGE(t *testing.T) {
	t.Run("imageId_required", func(t *testing.T) {
		t.Cleanup(func() {})
		msg := types.Message{To: "01012345678", Text: "text", KakaoOptions: freeBMSKakaoOptions("IMAGE")}
		errs := validateBMS(&msg, 0, "BMS_IMAGE", Options{})
		found := false
		for _, e := range errs {
			if e.Field == "imageId" {
				found = true
			}
		}
		if !found {
			t.Error("expected imageId error for BMS_IMAGE without image")
		}
	})

	t.Run("imageId_present_ok", func(t *testing.T) {
		t.Cleanup(func() {})
		msg := types.Message{To: "01012345678", Text: "text", ImageID: "img-1", KakaoOptions: freeBMSKakaoOptions("IMAGE")}
		errs := validateBMS(&msg, 0, "BMS_IMAGE", Options{})
		for _, e := range errs {
			if e.Field == "imageId" {
				t.Errorf("unexpected imageId error: %v", e)
			}
		}
	})
}

func TestValidateBMS_WIDE(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		wantField string
	}{
		{name: "76chars_ok", text: strings.Repeat("가", 76)},
		{name: "77chars_fail", text: strings.Repeat("가", 77), wantField: "text"},
		{name: "1newline_ok", text: "line1\nline2"},
		{name: "2newlines_fail", text: "line1\nline2\nline3", wantField: "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			msg := types.Message{To: "01012345678", Text: tt.text, ImageID: "img-1", KakaoOptions: freeBMSKakaoOptions("WIDE")}
			errs := validateBMS(&msg, 0, "BMS_WIDE", Options{})
			if tt.wantField != "" {
				found := false
				for _, e := range errs {
					if e.Field == tt.wantField {
						found = true
					}
				}
				if !found {
					t.Errorf("expected error on %q, got: %v", tt.wantField, errs)
				}
			}
		})
	}

	t.Run("buttons_max_2", func(t *testing.T) {
		t.Cleanup(func() {})
		ko := freeBMSKakaoOptions("WIDE")
		ko.Buttons = make([]types.KakaoButton, 3)
		msg := types.Message{To: "01012345678", Text: "text", ImageID: "img-1", KakaoOptions: ko}
		errs := validateBMS(&msg, 0, "BMS_WIDE", Options{})
		found := false
		for _, e := range errs {
			if e.Field == "kakaoOptions.buttons" {
				found = true
			}
		}
		if !found {
			t.Error("expected button count error for 3 buttons in BMS_WIDE")
		}
	})

	t.Run("button_name_max_8chars", func(t *testing.T) {
		t.Cleanup(func() {})
		ko := freeBMSKakaoOptions("WIDE")
		ko.Buttons = []types.KakaoButton{{ButtonName: strings.Repeat("가", 9)}}
		msg := types.Message{To: "01012345678", Text: "text", ImageID: "img-1", KakaoOptions: ko}
		errs := validateBMS(&msg, 0, "BMS_WIDE", Options{})
		found := false
		for _, e := range errs {
			if strings.Contains(e.Field, "buttonName") {
				found = true
			}
		}
		if !found {
			t.Error("expected button name length error")
		}
	})
}

func TestValidateBMS_COMMERCE(t *testing.T) {
	t.Run("imageId_required", func(t *testing.T) {
		t.Cleanup(func() {})
		ko := freeBMSKakaoOptions("COMMERCE")
		ko.Buttons = []types.KakaoButton{{ButtonType: "WL", ButtonName: "클릭"}}
		msg := types.Message{To: "01012345678", KakaoOptions: ko}
		errs := validateBMS(&msg, 0, "BMS_COMMERCE", Options{})
		found := false
		for _, e := range errs {
			if e.Field == "imageId" {
				found = true
			}
		}
		if !found {
			t.Error("expected imageId error")
		}
	})

	t.Run("buttons_min_1", func(t *testing.T) {
		t.Cleanup(func() {})
		ko := freeBMSKakaoOptions("COMMERCE")
		msg := types.Message{To: "01012345678", ImageID: "img-1", KakaoOptions: ko}
		errs := validateBMS(&msg, 0, "BMS_COMMERCE", Options{})
		found := false
		for _, e := range errs {
			if e.Field == "kakaoOptions.buttons" {
				found = true
			}
		}
		if !found {
			t.Error("expected min button error")
		}
	})

	t.Run("buttons_max_2", func(t *testing.T) {
		t.Cleanup(func() {})
		ko := freeBMSKakaoOptions("COMMERCE")
		ko.Buttons = make([]types.KakaoButton, 3)
		msg := types.Message{To: "01012345678", ImageID: "img-1", KakaoOptions: ko}
		errs := validateBMS(&msg, 0, "BMS_COMMERCE", Options{})
		found := false
		for _, e := range errs {
			if e.Field == "kakaoOptions.buttons" && strings.Contains(e.Message, "최대 2개") {
				found = true
			}
		}
		if !found {
			t.Error("expected max button error")
		}
	})

	t.Run("only_WL_AL_allowed", func(t *testing.T) {
		t.Cleanup(func() {})
		ko := freeBMSKakaoOptions("COMMERCE")
		ko.Buttons = []types.KakaoButton{{ButtonType: "BK", ButtonName: "키워드"}}
		msg := types.Message{To: "01012345678", ImageID: "img-1", KakaoOptions: ko}
		errs := validateBMS(&msg, 0, "BMS_COMMERCE", Options{})
		found := false
		for _, e := range errs {
			if strings.Contains(e.Field, "buttonType") {
				found = true
			}
		}
		if !found {
			t.Error("expected buttonType error for non-WL/AL")
		}
	})

	t.Run("empty_buttonType_rejected", func(t *testing.T) {
		t.Cleanup(func() {})
		ko := freeBMSKakaoOptions("COMMERCE")
		ko.Buttons = []types.KakaoButton{{ButtonType: "", ButtonName: "클릭"}}
		msg := types.Message{To: "01012345678", ImageID: "img-1", KakaoOptions: ko}
		errs := validateBMS(&msg, 0, "BMS_COMMERCE", Options{})
		found := false
		for _, e := range errs {
			if strings.Contains(e.Field, "buttonType") {
				found = true
			}
		}
		if !found {
			t.Error("expected buttonType error for empty buttonType")
		}
	})
}

func TestValidateBMS_PREMIUM_VIDEO(t *testing.T) {
	t.Run("text_76chars_ok", func(t *testing.T) {
		t.Cleanup(func() {})
		msg := types.Message{To: "01012345678", Text: strings.Repeat("가", 76), KakaoOptions: freeBMSKakaoOptions("PREMIUM_VIDEO")}
		errs := validateBMS(&msg, 0, "BMS_PREMIUM_VIDEO", Options{})
		for _, e := range errs {
			if e.Field == "text" {
				t.Errorf("unexpected text error: %v", e)
			}
		}
	})

	t.Run("text_77chars_fail", func(t *testing.T) {
		t.Cleanup(func() {})
		msg := types.Message{To: "01012345678", Text: strings.Repeat("가", 77), KakaoOptions: freeBMSKakaoOptions("PREMIUM_VIDEO")}
		errs := validateBMS(&msg, 0, "BMS_PREMIUM_VIDEO", Options{})
		found := false
		for _, e := range errs {
			if e.Field == "text" {
				found = true
			}
		}
		if !found {
			t.Error("expected text length error")
		}
	})

	t.Run("buttons_max_1", func(t *testing.T) {
		t.Cleanup(func() {})
		ko := freeBMSKakaoOptions("PREMIUM_VIDEO")
		ko.Buttons = make([]types.KakaoButton, 2)
		msg := types.Message{To: "01012345678", KakaoOptions: ko}
		errs := validateBMS(&msg, 0, "BMS_PREMIUM_VIDEO", Options{})
		found := false
		for _, e := range errs {
			if e.Field == "kakaoOptions.buttons" {
				found = true
			}
		}
		if !found {
			t.Error("expected button count error")
		}
	})
}

func TestValidateBMS_TargetingValues(t *testing.T) {
	validValues := []string{"I", "M", "N"}
	for _, v := range validValues {
		t.Run(fmt.Sprintf("targeting_%s_ok", v), func(t *testing.T) {
			t.Cleanup(func() {})
			ko := &types.KakaoOptions{
				PfID: "KA01PF", TemplateID: "KA01BP_001",
				BMS: &types.KakaoBMSOptions{ChatBubbleType: "TEXT", Targeting: v},
			}
			msg := types.Message{To: "01012345678", Text: "content", KakaoOptions: ko}
			errs := validateBMS(&msg, 0, "BMS_TEXT", Options{})
			for _, e := range errs {
				if e.Field == "kakaoOptions.bms.targeting" {
					t.Errorf("unexpected targeting error for %q: %v", v, e)
				}
			}
		})
	}
}
