package types

import (
	"strconv"
	"strings"
)

// FormatThousands renders an int with thousand separators. Negative numbers preserve '-'.
func FormatThousands(n int) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		if neg {
			return "-" + s
		}
		return s
	}
	head := len(s) % 3
	var b strings.Builder
	b.Grow(len(s) + len(s)/3 + 1)
	if neg {
		b.WriteByte('-')
	}
	if head > 0 {
		b.WriteString(s[:head])
		if len(s) > head {
			b.WriteByte(',')
		}
	}
	for i := head; i < len(s); i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < len(s) {
			b.WriteByte(',')
		}
	}
	return b.String()
}
