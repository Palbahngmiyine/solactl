package validation

import (
	"strconv"
	"strings"
	"testing"

	"github.com/solapi/solactl/pkg/types"
)

func TestValidateCommon_ValidMessage(t *testing.T) {
	t.Cleanup(func() {})
	msg := types.Message{To: "01012345678", From: "01011112222", Text: "hello"}
	errs := validateCommon(&msg, 0, Options{})
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateCommon_EmptyTo(t *testing.T) {
	t.Cleanup(func() {})
	msg := types.Message{To: "", Text: "hello"}
	errs := validateCommon(&msg, 0, Options{})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Code != "1010" || errs[0].Field != "to" {
		t.Errorf("unexpected error: %+v", errs[0])
	}
}

func TestValidateCommon_InvalidPhone(t *testing.T) {
	t.Cleanup(func() {})
	msg := types.Message{To: "abc", Text: "hello"}
	errs := validateCommon(&msg, 0, Options{})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Field != "to" {
		t.Errorf("expected field 'to', got %q", errs[0].Field)
	}
}

func TestValidateCommon_PhoneNormalized(t *testing.T) {
	t.Cleanup(func() {})
	msg := types.Message{To: "010-1234-5678", Text: "hello"}
	errs := validateCommon(&msg, 0, Options{})
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
	if msg.To != "01012345678" {
		t.Errorf("To not normalized: got %q, want %q", msg.To, "01012345678")
	}
}

func TestValidateCommon_CountryPopulated(t *testing.T) {
	t.Cleanup(func() {})
	msg := types.Message{To: "+82-10-1234-5678", Text: "hello"}
	errs := validateCommon(&msg, 0, Options{})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if msg.Country != "82" {
		t.Errorf("msg.Country = %q, want %q", msg.Country, "82")
	}
	if msg.To != "01012345678" {
		t.Errorf("msg.To = %q, want 01012345678", msg.To)
	}
}

func TestValidateCommon_NonKoreanCountryPassthrough(t *testing.T) {
	t.Cleanup(func() {})
	msg := types.Message{To: "+1-555-123-4567", Text: "hello"}
	errs := validateCommon(&msg, 0, Options{})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	// Non-Korean numbers: country stays empty (pass-through)
	if msg.Country != "" {
		t.Errorf("msg.Country = %q, want empty for non-Korean", msg.Country)
	}
	if msg.To != "15551234567" {
		t.Errorf("msg.To = %q, want 15551234567", msg.To)
	}
}

func TestValidateFrom_Required(t *testing.T) {
	tests := []struct {
		name     string
		from     string
		required bool
		wantErr  bool
	}{
		{name: "required_present", from: "01012345678", required: true, wantErr: false},
		{name: "required_absent", from: "", required: true, wantErr: true},
		{name: "optional_absent", from: "", required: false, wantErr: false},
		{name: "optional_present", from: "01012345678", required: false, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			msg := types.Message{From: tt.from}
			errs := validateFrom(&msg, 0, tt.required)
			if (len(errs) > 0) != tt.wantErr {
				t.Errorf("validateFrom(from=%q, required=%v) errors=%v, wantErr=%v",
					tt.from, tt.required, errs, tt.wantErr)
			}
		})
	}
}

func TestValidateCustomFields(t *testing.T) {
	tests := []struct {
		name    string
		fields  map[string]string
		wantN   int
	}{
		{name: "nil_fields", fields: nil, wantN: 0},
		{name: "empty_fields", fields: map[string]string{}, wantN: 0},
		{name: "valid_single", fields: map[string]string{"key": "value"}, wantN: 0},
		{name: "valid_10_fields", fields: makeNFields(10), wantN: 0},
		{name: "exceeds_10_fields", fields: makeNFields(11), wantN: 1},
		{name: "key_30chars_ok", fields: map[string]string{strings.Repeat("가", 30): "v"}, wantN: 0},
		{name: "key_31chars_fail", fields: map[string]string{strings.Repeat("가", 31): "v"}, wantN: 1},
		{name: "value_1000chars_ok", fields: map[string]string{"k": strings.Repeat("가", 1000)}, wantN: 0},
		{name: "value_1001chars_fail", fields: map[string]string{"k": strings.Repeat("가", 1001)}, wantN: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			errs := validateCustomFields(tt.fields, 0)
			if len(errs) != tt.wantN {
				t.Errorf("validateCustomFields() got %d errors, want %d: %v", len(errs), tt.wantN, errs)
			}
		})
	}
}

func TestCheckDuplicateRecipients(t *testing.T) {
	tests := []struct {
		name  string
		msgs  []types.Message
		wantN int
	}{
		{
			name:  "no_duplicates",
			msgs:  []types.Message{{To: "01011111111"}, {To: "01022222222"}},
			wantN: 0,
		},
		{
			name:  "one_duplicate",
			msgs:  []types.Message{{To: "01011111111"}, {To: "01011111111"}},
			wantN: 1,
		},
		{
			name: "multiple_duplicates",
			msgs: []types.Message{
				{To: "01011111111"},
				{To: "01022222222"},
				{To: "01011111111"},
				{To: "01022222222"},
			},
			wantN: 2,
		},
		{
			name:  "empty_to_skipped",
			msgs:  []types.Message{{To: ""}, {To: ""}},
			wantN: 0,
		},
		{
			name:  "single_message",
			msgs:  []types.Message{{To: "01011111111"}},
			wantN: 0,
		},
		{
			name:  "empty_list",
			msgs:  []types.Message{},
			wantN: 0,
		},
		{
			name:  "nil_list",
			msgs:  nil,
			wantN: 0,
		},
		{
			name: "triple_same_number",
			msgs: []types.Message{
				{To: "01011111111"},
				{To: "01011111111"},
				{To: "01011111111"},
			},
			wantN: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			errs := checkDuplicateRecipients(tt.msgs)
			if len(errs) != tt.wantN {
				t.Errorf("checkDuplicateRecipients() got %d errors, want %d: %v",
					len(errs), tt.wantN, errs)
			}
			// Verify error code
			for _, e := range errs {
				if e.Code != "1026" {
					t.Errorf("expected code 1026, got %q", e.Code)
				}
			}
		})
	}
}

func TestValidationErrors_Error(t *testing.T) {
	t.Cleanup(func() {})

	t.Run("empty", func(t *testing.T) {
		var ve ValidationErrors
		if ve.Error() != "" {
			t.Errorf("empty ValidationErrors.Error() = %q, want empty", ve.Error())
		}
	})

	t.Run("single", func(t *testing.T) {
		ve := ValidationErrors{{Index: 0, Field: "to", Code: "1010", Message: "invalid"}}
		got := ve.Error()
		if !strings.Contains(got, "검증 오류 1건") {
			t.Errorf("Error() = %q, want containing '검증 오류 1건'", got)
		}
		if !strings.Contains(got, "[0] to (1010): invalid") {
			t.Errorf("Error() = %q, want containing error detail", got)
		}
	})

	t.Run("multiple", func(t *testing.T) {
		ve := ValidationErrors{
			{Index: 0, Field: "to", Code: "1010", Message: "a"},
			{Index: 1, Field: "from", Code: "1010", Message: "b"},
		}
		got := ve.Error()
		if !strings.Contains(got, "검증 오류 2건") {
			t.Errorf("Error() = %q, want containing '검증 오류 2건'", got)
		}
	})
}

func TestValidationErrors_HasErrors(t *testing.T) {
	t.Cleanup(func() {})

	var empty ValidationErrors
	if empty.HasErrors() {
		t.Error("empty ValidationErrors.HasErrors() = true, want false")
	}

	nonempty := ValidationErrors{{Index: 0, Field: "to", Code: "1010", Message: "x"}}
	if !nonempty.HasErrors() {
		t.Error("non-empty ValidationErrors.HasErrors() = false, want true")
	}
}


func makeNFields(n int) map[string]string {
	m := make(map[string]string, n)
	for i := 0; i < n; i++ {
		m["key"+strconv.Itoa(i)] = "val"
	}
	return m
}
