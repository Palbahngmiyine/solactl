package validation

import (
	"strings"
	"testing"

	"github.com/solapi/solactl/pkg/types"
)

func validRCSMsg(text string) types.Message {
	return types.Message{
		To: "01012345678", From: "01011112222", Text: text,
		RCSOptions: &types.RCSOptions{BrandID: "brand-1"},
	}
}

func TestValidateRCS_SMS(t *testing.T) {
	tests := []struct {
		name      string
		msg       types.Message
		wantN     int
		wantField string
	}{
		{
			name:  "valid",
			msg:   validRCSMsg("짧은 메시지"),
			wantN: 0,
		},
		{
			name:  "100chars_ok",
			msg:   validRCSMsg(strings.Repeat("가", 100)),
			wantN: 0,
		},
		{
			name:      "101chars_fail",
			msg:       validRCSMsg(strings.Repeat("가", 101)),
			wantN:     1,
			wantField: "text",
		},
		{
			name: "text_empty",
			msg: types.Message{
				To: "01012345678", From: "01011112222",
				RCSOptions: &types.RCSOptions{BrandID: "brand-1"},
			},
			wantN:     1,
			wantField: "text",
		},
		{
			name: "from_missing",
			msg: types.Message{
				To: "01012345678", Text: "text",
				RCSOptions: &types.RCSOptions{BrandID: "brand-1"},
			},
			wantN:     1,
			wantField: "from",
		},
		{
			name: "brandId_missing",
			msg: types.Message{
				To: "01012345678", From: "01011112222", Text: "text",
				RCSOptions: &types.RCSOptions{},
			},
			wantN:     1,
			wantField: "rcsOptions.brandId",
		},
		{
			name: "imageId_forbidden",
			msg: types.Message{
				To: "01012345678", From: "01011112222", Text: "text", ImageID: "img-1",
				RCSOptions: &types.RCSOptions{BrandID: "brand-1"},
			},
			wantN:     1,
			wantField: "imageId",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			errs := validateRCS(&tt.msg, 0, "RCS_SMS", Options{})
			if len(errs) != tt.wantN {
				t.Errorf("got %d errors, want %d: %v", len(errs), tt.wantN, errs)
			}
			if tt.wantField != "" && len(errs) > 0 {
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
}

func TestValidateRCS_LMS(t *testing.T) {
	tests := []struct {
		name      string
		msg       types.Message
		opts      Options
		wantN     int
		wantField string
	}{
		{
			name:  "valid",
			msg:   validRCSMsg(strings.Repeat("가", 200)),
			wantN: 0,
		},
		{
			name:  "1300chars_ok",
			msg:   validRCSMsg(strings.Repeat("가", 1300)),
			wantN: 0,
		},
		{
			name:      "1301chars_fail",
			msg:       validRCSMsg(strings.Repeat("가", 1301)),
			wantN:     1,
			wantField: "text",
		},
		{
			name: "subject_30chars_ok",
			msg: func() types.Message {
				m := validRCSMsg("text")
				m.Subject = strings.Repeat("가", 30)
				return m
			}(),
			wantN: 0,
		},
		{
			name: "subject_31chars_strict_fail",
			msg: func() types.Message {
				m := validRCSMsg("text")
				m.Subject = strings.Repeat("가", 31)
				return m
			}(),
			opts:      Options{Strict: true},
			wantN:     1,
			wantField: "subject",
		},
		{
			name: "subject_31chars_nonstrict_ok",
			msg: func() types.Message {
				m := validRCSMsg("text")
				m.Subject = strings.Repeat("가", 31)
				return m
			}(),
			opts:  Options{Strict: false},
			wantN: 0,
		},
		{
			name: "imageId_forbidden",
			msg: func() types.Message {
				m := validRCSMsg("text")
				m.ImageID = "img-1"
				return m
			}(),
			wantN:     1,
			wantField: "imageId",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			errs := validateRCS(&tt.msg, 0, "RCS_LMS", tt.opts)
			if len(errs) != tt.wantN {
				t.Errorf("got %d errors, want %d: %v", len(errs), tt.wantN, errs)
			}
			if tt.wantField != "" && len(errs) > 0 {
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
}

func TestValidateRCS_MMS(t *testing.T) {
	tests := []struct {
		name      string
		msg       types.Message
		wantN     int
		wantField string
	}{
		{
			name: "valid",
			msg: func() types.Message {
				m := validRCSMsg("text")
				m.ImageID = "img-1"
				return m
			}(),
			wantN: 0,
		},
		{
			name:      "imageId_missing",
			msg:       validRCSMsg("text"),
			wantN:     1,
			wantField: "imageId",
		},
		{
			name: "text_1301chars_fail",
			msg: func() types.Message {
				m := validRCSMsg(strings.Repeat("가", 1301))
				m.ImageID = "img-1"
				return m
			}(),
			wantN:     1,
			wantField: "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			errs := validateRCS(&tt.msg, 0, "RCS_MMS", Options{})
			if len(errs) != tt.wantN {
				t.Errorf("got %d errors, want %d: %v", len(errs), tt.wantN, errs)
			}
			if tt.wantField != "" && len(errs) > 0 {
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
}

func TestValidateRCS_TPL(t *testing.T) {
	tests := []struct {
		name      string
		msg       types.Message
		wantN     int
		wantField string
	}{
		{
			name: "valid",
			msg: types.Message{
				To: "01012345678", From: "01011112222",
				RCSOptions: &types.RCSOptions{BrandID: "brand-1", TemplateID: "tpl-1"},
			},
			wantN: 0,
		},
		{
			name: "templateId_missing",
			msg: types.Message{
				To: "01012345678", From: "01011112222",
				RCSOptions: &types.RCSOptions{BrandID: "brand-1"},
			},
			wantN:     1,
			wantField: "rcsOptions.templateId",
		},
		{
			name: "text_2600chars_ok",
			msg: types.Message{
				To: "01012345678", From: "01011112222",
				Text:       strings.Repeat("가", 2600),
				RCSOptions: &types.RCSOptions{BrandID: "brand-1", TemplateID: "tpl-1"},
			},
			wantN: 0,
		},
		{
			name: "text_2601chars_fail",
			msg: types.Message{
				To: "01012345678", From: "01011112222",
				Text:       strings.Repeat("가", 2601),
				RCSOptions: &types.RCSOptions{BrandID: "brand-1", TemplateID: "tpl-1"},
			},
			wantN:     1,
			wantField: "text",
		},
		{
			name: "subject_60chars_ok",
			msg: types.Message{
				To: "01012345678", From: "01011112222",
				Subject:    strings.Repeat("가", 60),
				RCSOptions: &types.RCSOptions{BrandID: "brand-1", TemplateID: "tpl-1"},
			},
			wantN: 0,
		},
		{
			name: "subject_61chars_fail",
			msg: types.Message{
				To: "01012345678", From: "01011112222",
				Subject:    strings.Repeat("가", 61),
				RCSOptions: &types.RCSOptions{BrandID: "brand-1", TemplateID: "tpl-1"},
			},
			wantN:     1,
			wantField: "subject",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			errs := validateRCS(&tt.msg, 0, "RCS_TPL", Options{})
			if len(errs) != tt.wantN {
				t.Errorf("got %d errors, want %d: %v", len(errs), tt.wantN, errs)
			}
			if tt.wantField != "" && len(errs) > 0 {
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
}
