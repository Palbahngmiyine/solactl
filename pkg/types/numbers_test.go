package types

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

// TestFormatThousands는 boundary, sign, edge case를 모두 검증한다.
// 기존에는 pkg/progress의 wrapper로만 간접 검증되었으나, 함수를 numbers.go로
// 이동하면서 단위 테스트도 함께 numbers_test.go에 배치한다.
func TestFormatThousands(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want string
	}{
		{name: "zero", in: 0, want: "0"},
		{name: "single digit", in: 7, want: "7"},
		{name: "two digits", in: 42, want: "42"},
		{name: "three digits", in: 999, want: "999"},
		{name: "four digits — first comma", in: 1000, want: "1,000"},
		{name: "five digits", in: 12345, want: "12,345"},
		{name: "six digits", in: 123456, want: "123,456"},
		{name: "seven digits", in: 1234567, want: "1,234,567"},
		{name: "nine digits", in: 123456789, want: "123,456,789"},
		{name: "negative single digit", in: -7, want: "-7"},
		{name: "negative three digits", in: -999, want: "-999"},
		{name: "negative four digits", in: -1000, want: "-1,000"},
		{name: "negative large", in: -1234567890, want: "-1,234,567,890"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatThousands(tt.in); got != tt.want {
				t.Errorf("FormatThousands(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestFormatThousands_Boundary는 int 경계값에서 panic이나 잘못된 출력이
// 발생하지 않는지 검증한다 (math.MaxInt32 / MaxInt64).
func TestFormatThousands_Boundary(t *testing.T) {
	tests := []int{
		math.MaxInt32,
		math.MaxInt32 - 1,
		math.MinInt32 + 1, // MinInt는 negation 시 overflow하므로 +1을 사용
		math.MaxInt64,
		math.MinInt64 + 1,
	}
	for _, n := range tests {
		got := FormatThousands(n)
		// invariant 1: 부호 일관성
		if n < 0 && !strings.HasPrefix(got, "-") {
			t.Errorf("FormatThousands(%d) = %q, expected '-' prefix", n, got)
		}
		// invariant 2: 콤마 제외 시 strconv.Itoa(n)과 동일
		stripped := strings.ReplaceAll(got, ",", "")
		if stripped != strconv.Itoa(n) {
			t.Errorf("FormatThousands(%d) = %q, stripped %q != Itoa %q", n, got, stripped, strconv.Itoa(n))
		}
	}
}

// TestFormatThousands_CommaPositioning은 자릿수별 콤마 개수가 정확한지 검증한다.
// 4자리 → 1개, 7자리 → 2개, 10자리 → 3개.
func TestFormatThousands_CommaPositioning(t *testing.T) {
	cases := []struct {
		n          int
		wantCommas int
	}{
		{n: 1, wantCommas: 0},
		{n: 999, wantCommas: 0},
		{n: 1000, wantCommas: 1},
		{n: 999999, wantCommas: 1},
		{n: 1000000, wantCommas: 2},
		{n: 999999999, wantCommas: 2},
		{n: 1000000000, wantCommas: 3},
	}
	for _, c := range cases {
		got := FormatThousands(c.n)
		if cnt := strings.Count(got, ","); cnt != c.wantCommas {
			t.Errorf("FormatThousands(%d) = %q, comma count = %d, want %d", c.n, got, cnt, c.wantCommas)
		}
	}
}

// FuzzFormatThousands는 임의 입력에서 panic이 없고 invariant가 유지되는지 검증한다.
func FuzzFormatThousands(f *testing.F) {
	f.Add(0)
	f.Add(1)
	f.Add(-1)
	f.Add(1234567)
	f.Add(-987654321)
	f.Fuzz(func(t *testing.T, n int) {
		got := FormatThousands(n)
		// invariant: 콤마 제거 시 strconv.Itoa와 일치
		stripped := strings.ReplaceAll(got, ",", "")
		if stripped != strconv.Itoa(n) {
			t.Errorf("FormatThousands(%d) stripped %q != Itoa %q", n, stripped, strconv.Itoa(n))
		}
		// invariant: 결과는 비어있지 않음
		if got == "" {
			t.Errorf("FormatThousands(%d) returned empty", n)
		}
	})
}
