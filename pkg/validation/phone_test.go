package validation

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "digits_only", input: "01012345678", want: "01012345678"},
		{name: "with_dashes", input: "010-1234-5678", want: "01012345678"},
		{name: "with_spaces", input: "010 1234 5678", want: "01012345678"},
		{name: "with_parens", input: "(02) 999-9999", want: "029999999"},
		{name: "with_plus", input: "+82-10-1234-5678", want: "+821012345678"},
		{name: "mixed_formatting", input: "(+82) 10-1234-5678", want: "821012345678"},
		{name: "letters_stripped", input: "010abc5678", want: "0105678"},
		{name: "symbols_stripped", input: "010#1234*5678", want: "01012345678"},
		{name: "plus_only_at_start", input: "82+10", want: "8210"},
		{name: "non_ascii_stripped", input: "010가1234나5678", want: "01012345678"},
		{name: "empty", input: "", want: ""},
		{name: "only_formatting", input: "- () ", want: ""},
		{name: "only_non_ascii", input: "가나다", want: ""},
		{name: "already_clean", input: "12345", want: "12345"},
		{name: "long_number", input: "1234567890123456789012345", want: "1234567890123456789012345"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			got := NormalizePhone(tt.input)
			if got != tt.want {
				t.Errorf("NormalizePhone(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParsePhone_Valid(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantNumber  string
		wantCountry string
	}{
		{name: "korean_mobile", input: "01012345678", wantNumber: "01012345678", wantCountry: ""},
		{name: "korean_formatted", input: "010-1234-5678", wantNumber: "01012345678", wantCountry: ""},
		{name: "korean_landline", input: "02-999-9999", wantNumber: "029999999", wantCountry: ""},
		{name: "with_country_82", input: "+82-10-1234-5678", wantNumber: "01012345678", wantCountry: "82"},
		{name: "five_digits_minimum", input: "12345", wantNumber: "12345", wantCountry: ""},
		{name: "twenty_five_digits_max", input: strings.Repeat("1", 25), wantNumber: strings.Repeat("1", 25), wantCountry: ""},
		{name: "korean_special_15xx_no_prepend", input: "+82-1588-1234", wantNumber: "15881234", wantCountry: "82"},
		{name: "korean_special_16xx_no_prepend", input: "+82-1688-5678", wantNumber: "16885678", wantCountry: "82"},
		{name: "korean_special_18xx_no_prepend", input: "+82-1899-0000", wantNumber: "18990000", wantCountry: "82"},
		{name: "korean_mobile_with_country_prepend_0", input: "+82-10-9999-8888", wantNumber: "01099998888", wantCountry: "82"},
		{name: "us_number_passthrough", input: "+1-555-123-4567", wantNumber: "15551234567", wantCountry: ""},
		{name: "uk_number_passthrough", input: "+44-20-1234-5678", wantNumber: "442012345678", wantCountry: ""},
		// Parenthesized +82: the '(' strips the leading '+' position, so
		// the number is treated as domestic without country code.
		{name: "parens_82_domestic", input: "(+82) 10-1234-5678", wantNumber: "821012345678", wantCountry: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			number, country, err := ParsePhone(tt.input)
			if err != nil {
				t.Fatalf("ParsePhone(%q) unexpected error: %v", tt.input, err)
			}
			if number != tt.wantNumber {
				t.Errorf("ParsePhone(%q) number = %q, want %q", tt.input, number, tt.wantNumber)
			}
			if country != tt.wantCountry {
				t.Errorf("ParsePhone(%q) country = %q, want %q", tt.input, country, tt.wantCountry)
			}
		})
	}
}

func TestParsePhone_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMsg string
	}{
		{name: "empty", input: "", wantMsg: "비어 있습니다"},
		{name: "only_formatting", input: "- () ", wantMsg: "비어 있습니다"},
		{name: "only_non_ascii", input: "가나다", wantMsg: "비어 있습니다"},
		{name: "four_digits_too_short", input: "1234", wantMsg: "너무 짧습니다"},
		{name: "twenty_six_digits_too_long", input: strings.Repeat("1", 26), wantMsg: "너무 깁니다"},
		{name: "plus_only", input: "+", wantMsg: "파싱할 수 없습니다"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			_, _, err := ParsePhone(tt.input)
			if err == nil {
				t.Fatalf("ParsePhone(%q) expected error, got nil", tt.input)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("ParsePhone(%q) error = %q, want containing %q", tt.input, err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestParsePhone_Boundary(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "4_digits_fail", input: "1234", wantErr: true},
		{name: "5_digits_pass", input: "12345", wantErr: false},
		{name: "25_digits_pass", input: strings.Repeat("1", 25), wantErr: false},
		{name: "26_digits_fail", input: strings.Repeat("1", 26), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			_, _, err := ParsePhone(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePhone(%q) err = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestParsePhone_KoreaCountryCode82_PrependZero(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantNumber string
	}{
		// Numbers starting with 1 should get 0 prepended (except special 8-digit 15xx/16xx/18xx)
		{name: "10_mobile_prepend", input: "+82-10-1234-5678", wantNumber: "01012345678"},
		{name: "11_old_mobile_prepend", input: "+82-11-1234-5678", wantNumber: "01112345678"},
		{name: "12_prepend", input: "+82-12-345-6789", wantNumber: "0123456789"},
		// Special 8-digit numbers: no prepend
		{name: "1588_no_prepend", input: "+82-1588-1234", wantNumber: "15881234"},
		{name: "1600_no_prepend", input: "+82-1600-1234", wantNumber: "16001234"},
		{name: "1899_no_prepend", input: "+82-1899-0000", wantNumber: "18990000"},
		// 15xx but 9 digits: should prepend (not 8-digit special)
		{name: "1588_9digits_prepend", input: "+82-1588-12345", wantNumber: "0158812345"},
		// Number not starting with 1: no prepend
		{name: "2_landline_no_prepend", input: "+82-2-1234-5678", wantNumber: "212345678"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			number, _, err := ParsePhone(tt.input)
			if err != nil {
				t.Fatalf("ParsePhone(%q) unexpected error: %v", tt.input, err)
			}
			if number != tt.wantNumber {
				t.Errorf("ParsePhone(%q) = %q, want %q", tt.input, number, tt.wantNumber)
			}
		})
	}
}

func TestParsePhone_NoCountryCode_NoPrepend(t *testing.T) {
	// Without explicit +82 prefix, no 0-prepend happens even for numbers starting with 1.
	// The 0-prepend only applies when country code is explicitly "82".
	t.Cleanup(func() {})

	number, country, err := ParsePhone("1012345678")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if country != "" {
		t.Errorf("country = %q, want empty", country)
	}
	if number != "1012345678" {
		t.Errorf("number = %q, want 1012345678", number)
	}
}

func TestPhoneError(t *testing.T) {
	t.Cleanup(func() {})
	e := &PhoneError{Input: "abc", Reason: "잘못된 형식"}
	got := e.Error()
	if got != "잘못된 형식: abc" {
		t.Errorf("PhoneError.Error() = %q, want %q", got, "잘못된 형식: abc")
	}
}

func FuzzNormalizePhone(f *testing.F) {
	f.Add("")
	f.Add("01012345678")
	f.Add("+82-10-1234-5678")
	f.Add("(02) 999-9999")
	f.Add("가나다")
	f.Add(strings.Repeat("1", 100))

	f.Fuzz(func(t *testing.T, s string) {
		if !utf8.ValidString(s) {
			return
		}
		result := NormalizePhone(s)
		// Result should not contain formatting chars
		for _, r := range result {
			if r == '-' || r == ' ' || r == '(' || r == ')' {
				t.Errorf("NormalizePhone(%q) contains formatting char %q", s, string(r))
			}
			if r > 0x7F {
				t.Errorf("NormalizePhone(%q) contains non-ASCII char %q", s, string(r))
			}
		}
	})
}

func FuzzParsePhone(f *testing.F) {
	f.Add("01012345678")
	f.Add("+82-10-1234-5678")
	f.Add("")
	f.Add("1234")
	f.Add(strings.Repeat("9", 30))
	f.Add("abc")

	f.Fuzz(func(t *testing.T, s string) {
		if !utf8.ValidString(s) {
			return
		}
		// ParsePhone must never panic
		number, country, err := ParsePhone(s)
		if err != nil {
			return
		}
		// Valid results should have digits-only number
		for _, r := range number {
			if r < '0' || r > '9' {
				t.Errorf("ParsePhone(%q) number %q contains non-digit", s, number)
			}
		}
		// Country should be digits only or empty
		for _, r := range country {
			if r < '0' || r > '9' {
				t.Errorf("ParsePhone(%q) country %q contains non-digit", s, country)
			}
		}
		_ = number
		_ = country
	})
}
