package validation

import (
	"strings"
	"testing"

	"github.com/solapi/solactl/pkg/types"
)

func TestValidateSMS(t *testing.T) {
	tests := []struct {
		name    string
		msg     types.Message
		opts    Options
		wantN   int
		wantField string
	}{
		{
			name:  "valid_sms",
			msg:   types.Message{To: "01012345678", From: "01011112222", Text: "hello"},
			wantN: 0,
		},
		{
			name:  "valid_sms_exactly_90bytes",
			msg:   types.Message{To: "01012345678", From: "01011112222", Text: strings.Repeat("a", 90)},
			wantN: 0,
		},
		{
			name:      "text_91bytes_exceeds",
			msg:       types.Message{To: "01012345678", From: "01011112222", Text: strings.Repeat("a", 91)},
			wantN:     1,
			wantField: "text",
		},
		{
			name:  "korean_45chars_90bytes_ok",
			msg:   types.Message{To: "01012345678", From: "01011112222", Text: strings.Repeat("가", 45)},
			wantN: 0,
		},
		{
			name:      "korean_46chars_92bytes_exceeds",
			msg:       types.Message{To: "01012345678", From: "01011112222", Text: strings.Repeat("가", 46)},
			wantN:     1,
			wantField: "text",
		},
		{
			name:  "mixed_88ascii_1korean_90bytes_ok",
			msg:   types.Message{To: "01012345678", From: "01011112222", Text: strings.Repeat("a", 88) + "가"},
			wantN: 0,
		},
		{
			name:      "mixed_89ascii_1korean_91bytes_exceeds",
			msg:       types.Message{To: "01012345678", From: "01011112222", Text: strings.Repeat("a", 89) + "가"},
			wantN:     1,
			wantField: "text",
		},
		{
			name:      "text_empty",
			msg:       types.Message{To: "01012345678", From: "01011112222", Text: ""},
			wantN:     1,
			wantField: "text",
		},
		{
			name:      "from_missing",
			msg:       types.Message{To: "01012345678", From: "", Text: "hello"},
			wantN:     1,
			wantField: "from",
		},
		{
			name:      "subject_forbidden",
			msg:       types.Message{To: "01012345678", From: "01011112222", Text: "hello", Subject: "제목"},
			wantN:     1,
			wantField: "subject",
		},
		{
			name:      "imageId_forbidden",
			msg:       types.Message{To: "01012345678", From: "01011112222", Text: "hello", ImageID: "img-1"},
			wantN:     1,
			wantField: "imageId",
		},
		{
			name:  "multiple_errors",
			msg:   types.Message{To: "01012345678", From: "", Text: "", Subject: "제목", ImageID: "img-1"},
			wantN: 4, // from + text + subject + imageId
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			errs := validateSMS(&tt.msg, 0, tt.opts)
			if len(errs) != tt.wantN {
				t.Errorf("validateSMS() got %d errors, want %d: %v", len(errs), tt.wantN, errs)
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

func TestValidateLMS(t *testing.T) {
	tests := []struct {
		name    string
		msg     types.Message
		opts    Options
		wantN   int
		wantField string
	}{
		{
			name:  "valid_lms",
			msg:   types.Message{To: "01012345678", From: "01011112222", Text: strings.Repeat("a", 100)},
			wantN: 0,
		},
		{
			name:  "valid_lms_2000bytes",
			msg:   types.Message{To: "01012345678", From: "01011112222", Text: strings.Repeat("가", 1000)},
			wantN: 0,
		},
		{
			name:      "text_exceeds_2000bytes",
			msg:       types.Message{To: "01012345678", From: "01011112222", Text: strings.Repeat("가", 1001)},
			wantN:     1,
			wantField: "text",
		},
		{
			name:  "valid_with_subject_40bytes",
			msg:   types.Message{To: "01012345678", From: "01011112222", Text: "text", Subject: strings.Repeat("가", 20)},
			wantN: 0,
		},
		{
			name:      "subject_exceeds_40bytes_strict",
			msg:       types.Message{To: "01012345678", From: "01011112222", Text: "text", Subject: strings.Repeat("가", 21)},
			opts:      Options{Strict: true},
			wantN:     1,
			wantField: "subject",
		},
		{
			name:  "subject_exceeds_40bytes_nonstrict_no_error",
			msg:   types.Message{To: "01012345678", From: "01011112222", Text: "text", Subject: strings.Repeat("가", 21)},
			opts:  Options{Strict: false},
			wantN: 0,
		},
		{
			name:      "subject_required_in_strict",
			msg:       types.Message{To: "01012345678", From: "01011112222", Text: "text"},
			opts:      Options{Strict: true},
			wantN:     1,
			wantField: "subject",
		},
		{
			name:  "subject_optional_in_nonstrict",
			msg:   types.Message{To: "01012345678", From: "01011112222", Text: "text"},
			opts:  Options{Strict: false},
			wantN: 0,
		},
		{
			name:      "imageId_forbidden",
			msg:       types.Message{To: "01012345678", From: "01011112222", Text: "text", ImageID: "img-1"},
			wantN:     1,
			wantField: "imageId",
		},
		{
			name:      "from_missing",
			msg:       types.Message{To: "01012345678", Text: "text"},
			wantN:     1,
			wantField: "from",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			errs := validateLMS(&tt.msg, 0, tt.opts)
			if len(errs) != tt.wantN {
				t.Errorf("validateLMS() got %d errors, want %d: %v", len(errs), tt.wantN, errs)
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

func TestValidateMMS(t *testing.T) {
	tests := []struct {
		name    string
		msg     types.Message
		opts    Options
		wantN   int
		wantField string
	}{
		{
			name:  "valid_mms",
			msg:   types.Message{To: "01012345678", From: "01011112222", Text: "text", ImageID: "img-1"},
			wantN: 0,
		},
		{
			name:      "imageId_missing",
			msg:       types.Message{To: "01012345678", From: "01011112222", Text: "text"},
			wantN:     1,
			wantField: "imageId",
		},
		{
			name:      "text_exceeds_2000bytes",
			msg:       types.Message{To: "01012345678", From: "01011112222", Text: strings.Repeat("가", 1001), ImageID: "img-1"},
			wantN:     1,
			wantField: "text",
		},
		{
			name:      "subject_required_strict",
			msg:       types.Message{To: "01012345678", From: "01011112222", Text: "text", ImageID: "img-1"},
			opts:      Options{Strict: true},
			wantN:     1,
			wantField: "subject",
		},
		{
			name:  "subject_optional_nonstrict",
			msg:   types.Message{To: "01012345678", From: "01011112222", Text: "text", ImageID: "img-1"},
			opts:  Options{Strict: false},
			wantN: 0,
		},
		{
			name:      "from_missing",
			msg:       types.Message{To: "01012345678", Text: "text", ImageID: "img-1"},
			wantN:     1,
			wantField: "from",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			errs := validateMMS(&tt.msg, 0, tt.opts)
			if len(errs) != tt.wantN {
				t.Errorf("validateMMS() got %d errors, want %d: %v", len(errs), tt.wantN, errs)
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
