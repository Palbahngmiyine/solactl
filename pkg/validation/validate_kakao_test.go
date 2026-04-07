package validation

import (
	"strings"
	"testing"

	"github.com/solapi/solactl/pkg/types"
)

func TestValidateATA(t *testing.T) {
	validKO := func() *types.KakaoOptions {
		return &types.KakaoOptions{PfID: "KA01PF_001", TemplateID: "TPL_001"}
	}

	tests := []struct {
		name      string
		msg       types.Message
		wantN     int
		wantField string
	}{
		{
			name: "valid_ata",
			msg: types.Message{
				To: "01012345678", Text: "알림톡 내용",
				KakaoOptions: validKO(),
			},
			wantN: 0,
		},
		{
			name: "valid_ata_1000chars",
			msg: types.Message{
				To: "01012345678", Text: strings.Repeat("가", 1000),
				KakaoOptions: validKO(),
			},
			wantN: 0,
		},
		{
			name: "text_exceeds_1000chars",
			msg: types.Message{
				To: "01012345678", Text: strings.Repeat("가", 1001),
				KakaoOptions: validKO(),
			},
			wantN:     1,
			wantField: "text",
		},
		{
			name: "text_empty",
			msg: types.Message{
				To:           "01012345678",
				KakaoOptions: validKO(),
			},
			wantN:     1,
			wantField: "text",
		},
		{
			name: "kakaoOptions_nil",
			msg: types.Message{
				To: "01012345678", Text: "알림톡",
			},
			wantN:     1,
			wantField: "kakaoOptions",
		},
		{
			name: "pfId_missing",
			msg: types.Message{
				To: "01012345678", Text: "알림톡",
				KakaoOptions: &types.KakaoOptions{TemplateID: "TPL_001"},
			},
			wantN:     1,
			wantField: "kakaoOptions.pfId",
		},
		{
			name: "senderKey_as_alternative",
			msg: types.Message{
				To: "01012345678", Text: "알림톡",
				KakaoOptions: &types.KakaoOptions{SenderKey: "SK_001", TemplateID: "TPL_001"},
			},
			wantN: 0,
		},
		{
			name: "templateId_missing",
			msg: types.Message{
				To: "01012345678", Text: "알림톡",
				KakaoOptions: &types.KakaoOptions{PfID: "KA01PF_001"},
			},
			wantN:     1,
			wantField: "kakaoOptions.templateId",
		},
		{
			name: "buttons_5_ok",
			msg: types.Message{
				To: "01012345678", Text: "알림톡",
				KakaoOptions: &types.KakaoOptions{
					PfID: "KA01PF_001", TemplateID: "TPL_001",
					Buttons: make([]types.KakaoButton, 5),
				},
			},
			wantN: 0,
		},
		{
			name: "buttons_6_exceeds",
			msg: types.Message{
				To: "01012345678", Text: "알림톡",
				KakaoOptions: &types.KakaoOptions{
					PfID: "KA01PF_001", TemplateID: "TPL_001",
					Buttons: make([]types.KakaoButton, 6),
				},
			},
			wantN:     1,
			wantField: "kakaoOptions.buttons",
		},
		{
			name: "from_optional_no_error",
			msg: types.Message{
				To: "01012345678", Text: "알림톡",
				KakaoOptions: validKO(),
				// From intentionally empty
			},
			wantN: 0,
		},
		{
			name: "from_present_ok",
			msg: types.Message{
				To: "01012345678", From: "01011112222", Text: "알림톡",
				KakaoOptions: validKO(),
			},
			wantN: 0,
		},
		{
			name: "multiple_errors_pfId_and_templateId",
			msg: types.Message{
				To: "01012345678", Text: "알림톡",
				KakaoOptions: &types.KakaoOptions{},
			},
			wantN: 2, // pfId + templateId
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			errs := validateATA(&tt.msg, 0, Options{})
			if len(errs) != tt.wantN {
				t.Errorf("validateATA() got %d errors, want %d: %v", len(errs), tt.wantN, errs)
			}
			if tt.wantField != "" && len(errs) > 0 {
				found := false
				for _, e := range errs {
					if e.Field == tt.wantField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error on field %q, got: %v", tt.wantField, errs)
				}
			}
		})
	}
}

func TestValidateATA_TextBoundary(t *testing.T) {
	ko := &types.KakaoOptions{PfID: "KA01PF_001", TemplateID: "TPL_001"}

	tests := []struct {
		name    string
		text    string
		wantErr bool
	}{
		{name: "999chars_ok", text: strings.Repeat("가", 999), wantErr: false},
		{name: "1000chars_ok", text: strings.Repeat("가", 1000), wantErr: false},
		{name: "1001chars_fail", text: strings.Repeat("가", 1001), wantErr: true},
		{name: "1000_ascii_ok", text: strings.Repeat("a", 1000), wantErr: false},
		{name: "1001_ascii_fail", text: strings.Repeat("a", 1001), wantErr: true},
		{name: "mixed_999korean_1ascii_ok", text: strings.Repeat("가", 999) + "a", wantErr: false},
		{name: "emoji_counts_as_1", text: strings.Repeat("😀", 1000), wantErr: false},
		{name: "emoji_1001_fail", text: strings.Repeat("😀", 1001), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			msg := types.Message{To: "01012345678", Text: tt.text, KakaoOptions: ko}
			errs := validateATA(&msg, 0, Options{})
			hasTextErr := false
			for _, e := range errs {
				if e.Field == "text" {
					hasTextErr = true
				}
			}
			if hasTextErr != tt.wantErr {
				t.Errorf("text length %d chars: hasTextErr=%v, want %v",
					GetRealTextLength(tt.text), hasTextErr, tt.wantErr)
			}
		})
	}
}
