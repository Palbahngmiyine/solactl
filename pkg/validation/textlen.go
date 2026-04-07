// Package validation provides client-side message validation for solactl CLI.
package validation

// GetTextLength calculates text length using EUC-KR-style byte counting.
// ASCII (0x00-0x7F) = 1 byte, everything else = 2 bytes.
// Used for SMS (≤90), LMS/MMS text (≤2000), Subject (≤40).
func GetTextLength(s string) int {
	n := 0
	for _, r := range s {
		if r <= 0x7F {
			n++
		} else {
			n += 2
		}
	}
	return n
}

// GetRealTextLength returns the Unicode character count (number of runes).
// Used for ATA (≤1000 chars), RCS_SMS (≤100), RCS_LMS (≤1300), RCS_MMS (≤1300), RCS_TPL (≤2600).
func GetRealTextLength(s string) int {
	return len([]rune(s))
}

// GetJSStringLength returns the equivalent of JavaScript's string.length,
// which counts UTF-16 code units. Characters in the BMP (U+0000–U+FFFF)
// count as 1; supplementary characters (U+10000+) count as 2 (surrogate pair).
// Used for all BMS content fields.
func GetJSStringLength(s string) int {
	n := 0
	for _, r := range s {
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
	}
	return n
}
