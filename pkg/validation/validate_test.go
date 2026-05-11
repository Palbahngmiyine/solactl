package validation

import (
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/solapi/solactl/pkg/types"
)

func TestValidateMessages_ValidSMS(t *testing.T) {
	t.Cleanup(func() {})
	msgs := []types.Message{
		{To: "01012345678", From: "01011112222", Text: "hello"},
	}
	errs := ValidateMessages(msgs, Options{AutoTypeDetect: true})
	if errs != nil {
		t.Errorf("expected no errors, got %v", errs)
	}
	if msgs[0].Type != "SMS" {
		t.Errorf("expected auto-detected type SMS, got %q", msgs[0].Type)
	}
}

func TestValidateMessages_AutoTypeDetect(t *testing.T) {
	tests := []struct {
		name     string
		msg      types.Message
		wantType string
	}{
		{
			name:     "sms",
			msg:      types.Message{To: "01012345678", From: "01011112222", Text: "short"},
			wantType: "SMS",
		},
		{
			name:     "lms_long_text",
			msg:      types.Message{To: "01012345678", From: "01011112222", Text: strings.Repeat("a", 91)},
			wantType: "LMS",
		},
		{
			name:     "mms_with_image",
			msg:      types.Message{To: "01012345678", From: "01011112222", Text: "text", ImageID: "img-1"},
			wantType: "MMS",
		},
		{
			name: "ata_with_kakao",
			msg: types.Message{
				To: "01012345678", Text: "알림톡",
				KakaoOptions: &types.KakaoOptions{PfID: "pf", TemplateID: "tpl"},
			},
			wantType: "ATA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			msgs := []types.Message{tt.msg}
			_ = ValidateMessages(msgs, Options{AutoTypeDetect: true})
			if msgs[0].Type != tt.wantType {
				t.Errorf("auto-detect = %q, want %q", msgs[0].Type, tt.wantType)
			}
		})
	}
}

func TestValidateMessages_NoAutoTypeDetect(t *testing.T) {
	t.Cleanup(func() {})
	msgs := []types.Message{
		{To: "01012345678", From: "01011112222", Text: "hello"},
	}
	_ = ValidateMessages(msgs, Options{AutoTypeDetect: false})
	if msgs[0].Type != "" {
		t.Errorf("type should remain empty when auto-detect disabled, got %q", msgs[0].Type)
	}
}

func TestValidateMessages_ExplicitType(t *testing.T) {
	t.Cleanup(func() {})
	msgs := []types.Message{
		{To: "01012345678", From: "01011112222", Text: "hello", Type: "LMS"},
	}
	errs := ValidateMessages(msgs, Options{AutoTypeDetect: true})
	if errs != nil {
		t.Errorf("expected no errors, got %v", errs)
	}
	// Type should remain as explicitly set
	if msgs[0].Type != "LMS" {
		t.Errorf("type should remain LMS, got %q", msgs[0].Type)
	}
}

func TestValidateMessages_MultipleMessages(t *testing.T) {
	t.Cleanup(func() {})
	msgs := []types.Message{
		{To: "01012345678", From: "01011112222", Text: "hello"},
		{To: "01099998888", From: "01011112222", Text: "world"},
	}
	errs := ValidateMessages(msgs, Options{AutoTypeDetect: true})
	if errs != nil {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateMessages_MultipleErrors(t *testing.T) {
	t.Cleanup(func() {})
	msgs := []types.Message{
		{To: "", Text: ""},         // to empty, text empty, from missing
		{To: "abc", Text: "hello"}, // invalid phone
	}
	errs := ValidateMessages(msgs, Options{AutoTypeDetect: true})
	if errs == nil {
		t.Fatal("expected errors, got nil")
	}
	if len(errs) < 2 {
		t.Errorf("expected at least 2 errors, got %d", len(errs))
	}
}

func TestValidateMessages_DuplicateRecipients(t *testing.T) {
	t.Run("duplicates_rejected_by_default", func(t *testing.T) {
		t.Cleanup(func() {})
		msgs := []types.Message{
			{To: "01012345678", From: "01011112222", Text: "hello"},
			{To: "01012345678", From: "01011112222", Text: "world"},
		}
		errs := ValidateMessages(msgs, Options{AutoTypeDetect: true})
		if errs == nil {
			t.Fatal("expected duplicate error")
		}
		found := false
		for _, e := range errs {
			if e.Code == "1026" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected code 1026 for duplicates, got: %v", errs)
		}
	})

	t.Run("duplicates_allowed", func(t *testing.T) {
		t.Cleanup(func() {})
		msgs := []types.Message{
			{To: "01012345678", From: "01011112222", Text: "hello"},
			{To: "01012345678", From: "01011112222", Text: "world"},
		}
		errs := ValidateMessages(msgs, Options{AutoTypeDetect: true, AllowDuplicates: true})
		// Should not have duplicate errors (may have other errors)
		for _, e := range errs {
			if e.Code == "1026" {
				t.Errorf("should not have duplicate error when AllowDuplicates=true")
			}
		}
	})
}

func TestValidateMessages_PhoneNormalization(t *testing.T) {
	t.Cleanup(func() {})
	msgs := []types.Message{
		{To: "010-1234-5678", From: "01011112222", Text: "hello"},
	}
	errs := ValidateMessages(msgs, Options{AutoTypeDetect: true})
	if errs != nil {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if msgs[0].To != "01012345678" {
		t.Errorf("To not normalized: %q", msgs[0].To)
	}
}

func TestValidateMessages_EmptySlice(t *testing.T) {
	t.Cleanup(func() {})
	errs := ValidateMessages([]types.Message{}, Options{})
	if errs != nil {
		t.Errorf("expected nil for empty slice, got %v", errs)
	}
}

func TestValidateMessages_NilSlice(t *testing.T) {
	t.Cleanup(func() {})
	errs := ValidateMessages(nil, Options{})
	if errs != nil {
		t.Errorf("expected nil for nil slice, got %v", errs)
	}
}

func TestValidateMessages_Concurrent(t *testing.T) {
	t.Cleanup(func() {})

	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			msgs := []types.Message{
				{To: "01012345678", From: "01011112222", Text: "hello"},
			}
			_ = ValidateMessages(msgs, Options{AutoTypeDetect: true})
		})
	}
	wg.Wait()
}

func FuzzValidateMessages(f *testing.F) {
	f.Add("01012345678", "01011112222", "hello")
	f.Add("", "", "")
	f.Add("+82-10-1234-5678", "010", strings.Repeat("가", 100))
	f.Add("abc", "xyz", strings.Repeat("a", 200))

	f.Fuzz(func(t *testing.T, to, from, text string) {
		if !utf8.ValidString(to) || !utf8.ValidString(from) || !utf8.ValidString(text) {
			return
		}
		msgs := []types.Message{
			{To: to, From: from, Text: text},
		}
		// Must not panic
		_ = ValidateMessages(msgs, Options{AutoTypeDetect: true})
	})
}
