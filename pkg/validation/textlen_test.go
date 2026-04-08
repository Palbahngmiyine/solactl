package validation

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestGetTextLength(t *testing.T) {
	tests := []struct {
		name string
		input string
		want int
	}{
		{name: "empty", input: "", want: 0},
		{name: "ascii_single", input: "a", want: 1},
		{name: "ascii_90chars", input: strings.Repeat("a", 90), want: 90},
		{name: "ascii_91chars", input: strings.Repeat("a", 91), want: 91},
		{name: "korean_single", input: "가", want: 2},
		{name: "korean_45chars_boundary_90bytes", input: strings.Repeat("가", 45), want: 90},
		{name: "korean_46chars_boundary_92bytes", input: strings.Repeat("가", 46), want: 92},
		{name: "mixed_ascii_korean", input: "hello가나", want: 5 + 4},
		{name: "mixed_boundary_88ascii_1korean", input: strings.Repeat("a", 88) + "가", want: 90},
		{name: "mixed_boundary_89ascii_1korean", input: strings.Repeat("a", 89) + "가", want: 91},
		{name: "japanese_katakana", input: "アイウ", want: 6},
		{name: "chinese_char", input: "中文", want: 4},
		{name: "emoji_basic", input: "😀", want: 2},
		{name: "digits", input: "01012345678", want: 11},
		{name: "space", input: " ", want: 1},
		{name: "newline", input: "\n", want: 1},
		{name: "tab", input: "\t", want: 1},
		{name: "null_byte", input: "\x00", want: 1},
		{name: "del_0x7F", input: "\x7F", want: 1},
		{name: "0x80_boundary", input: "\u0080", want: 2},
		{name: "max_bmp", input: "\uFFFF", want: 2},
		{name: "supplementary_char", input: "𠀀", want: 2},
		{name: "lms_2000bytes", input: strings.Repeat("가", 1000), want: 2000},
		{name: "lms_2001bytes", input: strings.Repeat("가", 1000) + "a", want: 2001},
		{name: "subject_40bytes", input: strings.Repeat("가", 20), want: 40},
		{name: "subject_41bytes", input: strings.Repeat("가", 20) + "a", want: 41},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			got := GetTextLength(tt.input)
			if got != tt.want {
				t.Errorf("GetTextLength(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetRealTextLength(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{name: "empty", input: "", want: 0},
		{name: "ascii_single", input: "a", want: 1},
		{name: "korean_single", input: "가", want: 1},
		{name: "korean_1000chars", input: strings.Repeat("가", 1000), want: 1000},
		{name: "korean_1001chars", input: strings.Repeat("가", 1001), want: 1001},
		{name: "mixed", input: "hello가나다", want: 8},
		{name: "emoji_as_one_rune", input: "😀", want: 1},
		{name: "supplementary_char", input: "𠀀", want: 1},
		{name: "rcs_sms_100chars", input: strings.Repeat("가", 100), want: 100},
		{name: "rcs_sms_101chars", input: strings.Repeat("가", 101), want: 101},
		{name: "rcs_lms_1300chars", input: strings.Repeat("가", 1300), want: 1300},
		{name: "rcs_tpl_2600chars", input: strings.Repeat("가", 2600), want: 2600},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			got := GetRealTextLength(tt.input)
			if got != tt.want {
				t.Errorf("GetRealTextLength(%q) = %d, want %d", truncateForLog(tt.input), got, tt.want)
			}
		})
	}
}

func TestGetJSStringLength(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{name: "empty", input: "", want: 0},
		{name: "ascii_single", input: "a", want: 1},
		{name: "korean_single", input: "가", want: 1},
		{name: "bmp_char", input: "\uFFFF", want: 1},
		{name: "supplementary_emoji_surrogate_pair", input: "😀", want: 2},
		{name: "supplementary_cjk", input: "𠀀", want: 2},
		{name: "mixed_bmp_and_supplementary", input: "abc😀def", want: 8},
		{name: "multiple_supplementary", input: "😀😁😂", want: 6},
		{name: "korean_1300chars", input: strings.Repeat("가", 1300), want: 1300},
		{name: "bms_text_boundary", input: strings.Repeat("가", 1300), want: 1300},
		{name: "bms_wide_76chars", input: strings.Repeat("가", 76), want: 76},
		{name: "bms_wide_77chars", input: strings.Repeat("가", 77), want: 77},
		{name: "only_supplementary_1char", input: "𝄞", want: 2},
		{name: "combining_chars", input: "e\u0301", want: 2},    // e + combining accent = 2 BMP runes = 2 code units
		{name: "precomposed_char", input: "\u00E9", want: 1},   // precomposed é = 1 code unit
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {})
			got := GetJSStringLength(tt.input)
			if got != tt.want {
				t.Errorf("GetJSStringLength(%q) = %d, want %d", truncateForLog(tt.input), got, tt.want)
			}
		})
	}
}

func TestGetTextLength_VsUTF8ByteLength(t *testing.T) {
	// Verify that GetTextLength differs from UTF-8 byte length for non-ASCII
	t.Cleanup(func() {})

	korean := "가" // UTF-8: 3 bytes, EUC-KR style: 2 bytes
	utf8Len := len([]byte(korean))
	textLen := GetTextLength(korean)

	if utf8Len != 3 {
		t.Errorf("UTF-8 byte length of '가' = %d, want 3", utf8Len)
	}
	if textLen != 2 {
		t.Errorf("GetTextLength('가') = %d, want 2", textLen)
	}
	if utf8Len == textLen {
		t.Error("GetTextLength should differ from UTF-8 byte length for Korean chars")
	}
}

func TestGetJSStringLength_VsRuneCount(t *testing.T) {
	// Verify that GetJSStringLength differs from rune count for supplementary chars
	t.Cleanup(func() {})

	emoji := "😀" // 1 rune, but 2 UTF-16 code units
	runeCount := utf8.RuneCountInString(emoji)
	jsLen := GetJSStringLength(emoji)

	if runeCount != 1 {
		t.Errorf("rune count of '😀' = %d, want 1", runeCount)
	}
	if jsLen != 2 {
		t.Errorf("GetJSStringLength('😀') = %d, want 2", jsLen)
	}
}

func FuzzGetTextLength(f *testing.F) {
	f.Add("")
	f.Add("hello")
	f.Add("가나다")
	f.Add("hello가나다")
	f.Add("😀")
	f.Add("\x00\x7F\x80")

	f.Fuzz(func(t *testing.T, s string) {
		if !utf8.ValidString(s) {
			return
		}
		n := GetTextLength(s)
		if n < 0 {
			t.Errorf("GetTextLength returned negative: %d", n)
		}
		// Text length should be >= rune count (since non-ASCII runes count as 2)
		runeCount := utf8.RuneCountInString(s)
		if n < runeCount {
			t.Errorf("GetTextLength(%d) < rune count(%d)", n, runeCount)
		}
	})
}

func FuzzGetJSStringLength(f *testing.F) {
	f.Add("")
	f.Add("hello")
	f.Add("가나다")
	f.Add("😀😁😂")
	f.Add("𠀀")

	f.Fuzz(func(t *testing.T, s string) {
		if !utf8.ValidString(s) {
			return
		}
		n := GetJSStringLength(s)
		if n < 0 {
			t.Errorf("GetJSStringLength returned negative: %d", n)
		}
		// JS string length should be >= rune count
		runeCount := utf8.RuneCountInString(s)
		if n < runeCount {
			t.Errorf("GetJSStringLength(%d) < rune count(%d)", n, runeCount)
		}
	})
}

// truncateForLog truncates a string for test log output.
func truncateForLog(s string) string {
	if len(s) > 40 {
		return s[:40] + "..."
	}
	return s
}
