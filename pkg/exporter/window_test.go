package exporter

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// helper: UTC date-only.
func ut(y int, m time.Month, d, hh, mm, ss int) time.Time {
	return time.Date(y, m, d, hh, mm, ss, 0, time.UTC)
}

// helper: UTC midnight.
func mid(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestWindow_Label(t *testing.T) {
	t.Parallel()
	w := Window{Start: ut(2026, 4, 8, 13, 45, 0), End: ut(2026, 4, 9, 0, 0, 0)}
	if got, want := w.Label(), "2026-04-08"; got != want {
		t.Fatalf("Label()=%q want %q", got, want)
	}
}

func TestSplitWindows_SingleWindow_WithinLimit(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		start     time.Time
		end       time.Time
		want      int
		wantStart time.Time
		wantEnd   time.Time
	}{
		{
			name:      "5일 범위 단일",
			start:     mid(2026, 4, 2),
			end:       mid(2026, 4, 7),
			want:      1,
			wantStart: mid(2026, 4, 2),
			wantEnd:   mid(2026, 4, 7),
		},
		{
			name:      "정확히 7일 단일",
			start:     mid(2026, 4, 2),
			end:       mid(2026, 4, 9),
			want:      1,
			wantStart: mid(2026, 4, 2),
			wantEnd:   mid(2026, 4, 9),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := SplitWindows(tc.start, tc.end, 7)
			if len(got) != tc.want {
				t.Fatalf("len=%d want %d, got=%+v", len(got), tc.want, got)
			}
			if !got[0].Start.Equal(tc.wantStart) || !got[0].End.Equal(tc.wantEnd) {
				t.Fatalf("window=%+v want [%v,%v)", got[0], tc.wantStart, tc.wantEnd)
			}
		})
	}
}

func TestSplitWindows_OverLimit_Split(t *testing.T) {
	t.Parallel()
	// 7일 1초 초과 → 분할 경로.
	start := mid(2026, 4, 2)
	end := mid(2026, 4, 9).Add(1 * time.Second)
	got := SplitWindows(start, end, 7)
	// startFloor=04-02, endCeil=04-10 → 8 windows.
	if len(got) != 8 {
		t.Fatalf("len=%d want 8, got=%+v", len(got), got)
	}
	if !got[0].Start.Equal(mid(2026, 4, 2)) {
		t.Fatalf("first window start=%v want %v", got[0].Start, mid(2026, 4, 2))
	}
	if !got[len(got)-1].End.Equal(mid(2026, 4, 10)) {
		t.Fatalf("last window end=%v want %v", got[len(got)-1].End, mid(2026, 4, 10))
	}
	// 모든 윈도우는 1일짜리.
	for i, w := range got {
		if w.End.Sub(w.Start) != 24*time.Hour {
			t.Fatalf("window[%d] length=%v want 24h", i, w.End.Sub(w.Start))
		}
	}
}

func TestSplitWindows_MidnightAlignment(t *testing.T) {
	t.Parallel()
	// 같은 자정 정렬 검증: 비자정 입력은 floor/ceil로 보정.
	start := ut(2026, 4, 2, 10, 23, 0)
	end := ut(2026, 4, 15, 18, 45, 0)
	got := SplitWindows(start, end, 7)
	// 14일치 (04-02..04-16). 분할 경로 (raw length=13d8h22m > 7d).
	if len(got) != 14 {
		t.Fatalf("len=%d want 14", len(got))
	}
	if !got[0].Start.Equal(mid(2026, 4, 2)) || !got[0].End.Equal(mid(2026, 4, 3)) {
		t.Fatalf("first=%+v want [04-02,04-03)", got[0])
	}
	if !got[13].Start.Equal(mid(2026, 4, 15)) || !got[13].End.Equal(mid(2026, 4, 16)) {
		t.Fatalf("last=%+v want [04-15,04-16)", got[13])
	}
}

func TestSplitWindows_ExactMidnight_31Days(t *testing.T) {
	t.Parallel()
	// 운영 로그 시나리오: 31일치, 정확한 자정. 단순 분할 검증.
	start := mid(2026, 4, 2)
	end := mid(2026, 5, 3)
	got := SplitWindows(start, end, 7)
	if len(got) != 31 {
		t.Fatalf("len=%d want 31", len(got))
	}
	if !got[0].Start.Equal(mid(2026, 4, 2)) {
		t.Fatalf("first start=%v want %v", got[0].Start, mid(2026, 4, 2))
	}
	if got[0].Label() != "2026-04-02" {
		t.Fatalf("first label=%q want %q", got[0].Label(), "2026-04-02")
	}
	if !got[30].Start.Equal(mid(2026, 5, 2)) {
		t.Fatalf("last start=%v want %v", got[30].Start, mid(2026, 5, 2))
	}
	if !got[30].End.Equal(mid(2026, 5, 3)) {
		t.Fatalf("last end=%v want %v", got[30].End, mid(2026, 5, 3))
	}
}

func TestSplitWindows_BoundaryDays(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		start    time.Time
		end      time.Time
		wantLen  int
		wantPath string // "single" 또는 "split"
	}{
		{
			name:     "6일 23시간 단일",
			start:    mid(2026, 4, 2),
			end:      mid(2026, 4, 8).Add(23 * time.Hour),
			wantLen:  1,
			wantPath: "single",
		},
		{
			name:     "7일 정각 단일",
			start:    mid(2026, 4, 2),
			end:      mid(2026, 4, 9),
			wantLen:  1,
			wantPath: "single",
		},
		{
			name:     "7일 1초 분할 8개",
			start:    mid(2026, 4, 2),
			end:      mid(2026, 4, 9).Add(1 * time.Second),
			wantLen:  8,
			wantPath: "split",
		},
		{
			name:     "8일 분할 8개",
			start:    mid(2026, 4, 2),
			end:      mid(2026, 4, 10),
			wantLen:  8,
			wantPath: "split",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := SplitWindows(tc.start, tc.end, 7)
			if len(got) != tc.wantLen {
				t.Fatalf("len=%d want %d (path=%s)", len(got), tc.wantLen, tc.wantPath)
			}
		})
	}
}

func TestSplitWindows_EmptyOrInvalid(t *testing.T) {
	t.Parallel()
	t0 := mid(2026, 4, 2)
	cases := []struct {
		name  string
		start time.Time
		end   time.Time
	}{
		{"start==end 자정 동일", t0, t0},
		{"start>end 역전", t0.Add(24 * time.Hour), t0},
		{"start==end 비자정 동일", ut(2026, 4, 2, 10, 0, 0), ut(2026, 4, 2, 10, 0, 0)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := SplitWindows(tc.start, tc.end, 7); len(got) != 0 {
				t.Fatalf("len=%d want 0, got=%+v", len(got), got)
			}
		})
	}
}

func TestSplitWindows_SameDay_DifferentHours(t *testing.T) {
	t.Parallel()
	// 같은 날 다른 시각 → 단일 윈도우 [floor, ceil) = [00:00, 다음날 00:00).
	got := SplitWindows(ut(2026, 4, 2, 10, 0, 0), ut(2026, 4, 2, 14, 0, 0), 7)
	if len(got) != 1 {
		t.Fatalf("len=%d want 1", len(got))
	}
	if !got[0].Start.Equal(mid(2026, 4, 2)) {
		t.Fatalf("start=%v want %v", got[0].Start, mid(2026, 4, 2))
	}
	if !got[0].End.Equal(mid(2026, 4, 3)) {
		t.Fatalf("end=%v want %v", got[0].End, mid(2026, 4, 3))
	}
}

func TestSplitWindows_MaxWindowDaysDefault(t *testing.T) {
	t.Parallel()
	// maxWindowDays <= 0 → 7 default.
	got := SplitWindows(mid(2026, 4, 2), mid(2026, 4, 9), 0)
	if len(got) != 1 {
		t.Fatalf("zero maxWindowDays should use default 7, len=%d", len(got))
	}
	got = SplitWindows(mid(2026, 4, 2), mid(2026, 4, 9), -5)
	if len(got) != 1 {
		t.Fatalf("negative maxWindowDays should use default 7, len=%d", len(got))
	}
}

func TestSplitWindows_DSTUnaffected(t *testing.T) {
	t.Parallel()
	// DST 전환은 KST/UTC 비교용. UTC 기준이므로 결과는 정확히 24h*N.
	// 2026-03-29 DST 전환일 (유럽), UTC 기준으로 검증.
	start := mid(2026, 3, 28)
	end := mid(2026, 3, 31)
	got := SplitWindows(start, end, 7)
	if len(got) != 1 {
		t.Fatalf("len=%d want 1 (3일 단일)", len(got))
	}
	if got[0].End.Sub(got[0].Start) != 72*time.Hour {
		t.Fatalf("length=%v want 72h (DST 영향 없음)", got[0].End.Sub(got[0].Start))
	}
}

// --- ValidateDateRange ----------------------------------------------------

func TestValidateDateRange_LookbackExceeded(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)
	// 180일 전 = 2025-11-12 00:00 UTC.
	cases := []struct {
		name    string
		start   time.Time
		wantErr error
	}{
		{"경계: 정확히 180일 전 OK", mid(2025, 11, 12), nil},
		{"초과: 180일+1초 전", mid(2025, 11, 12).Add(-1 * time.Second), ErrLookbackExceeded},
		{"훨씬 과거", mid(2024, 1, 1), ErrLookbackExceeded},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ValidateDateRange(tc.start, mid(2026, 5, 1), now, 180)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("want nil err, got %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("want errors.Is %v, got %v", tc.wantErr, err)
			}
			// cutoff 날짜가 메시지에 포함되어야 함.
			if !strings.Contains(err.Error(), "2025-11-12") {
				t.Fatalf("err message missing cutoff date: %v", err)
			}
		})
	}
}

func TestValidateDateRange_FutureEndClamp(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)
	end := mid(2026, 5, 12)
	effEnd, err := ValidateDateRange(mid(2026, 4, 1), end, now, 180)
	if err != nil {
		t.Fatalf("clamp should not error, got %v", err)
	}
	if !effEnd.Equal(now) {
		t.Fatalf("effEnd=%v want now=%v (clamped)", effEnd, now)
	}
	if effEnd.Equal(end) {
		t.Fatalf("effEnd must differ from end to signal clamp")
	}
}

func TestValidateDateRange_NoClamp_EndEqualNow(t *testing.T) {
	t.Parallel()
	// end == now는 clamp 아님.
	now := mid(2026, 5, 11)
	effEnd, err := ValidateDateRange(mid(2026, 4, 1), now, now, 180)
	if err != nil {
		t.Fatalf("got err %v", err)
	}
	if !effEnd.Equal(now) {
		t.Fatalf("effEnd=%v want %v", effEnd, now)
	}
}

func TestValidateDateRange_StartAfterEnd(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)
	cases := []struct {
		name        string
		start, end  time.Time
		wantErrType error
	}{
		{"start==end 동일", mid(2026, 5, 1), mid(2026, 5, 1), ErrStartAfterEnd},
		{"start>end 역전", mid(2026, 5, 5), mid(2026, 5, 1), ErrStartAfterEnd},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ValidateDateRange(tc.start, tc.end, now, 180)
			if !errors.Is(err, tc.wantErrType) {
				t.Fatalf("want errors.Is %v, got %v", tc.wantErrType, err)
			}
		})
	}
}

func TestValidateDateRange_ValidationOrder(t *testing.T) {
	t.Parallel()
	// lookback 위반 + start>=end 동시 → lookback 에러 우선.
	now := mid(2026, 5, 11)
	start := mid(2024, 1, 1)
	end := mid(2024, 1, 1) // start==end
	_, err := ValidateDateRange(start, end, now, 180)
	if !errors.Is(err, ErrLookbackExceeded) {
		t.Fatalf("want lookback err first, got %v", err)
	}
}

func TestValidateDateRange_DefaultLookback(t *testing.T) {
	t.Parallel()
	// maxLookbackDays<=0 → DefaultMaxLookbackDays.
	now := mid(2026, 5, 11)
	// 200일 전 (default 180을 넘어야 reject).
	start := now.AddDate(0, 0, -200)
	_, err := ValidateDateRange(start, mid(2026, 5, 10), now, 0)
	if !errors.Is(err, ErrLookbackExceeded) {
		t.Fatalf("zero maxLookbackDays should use default 180, got err=%v", err)
	}
}

// --- ResumeToken ---------------------------------------------------------

func TestEncodeDecodeToken_RoundTrip(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   ResumeToken
	}{
		{
			name: "no state",
			in:   ResumeToken{Version: 1, Window: "2026-04-09"},
		},
		{
			name: "with state",
			in:   ResumeToken{Version: 1, Window: "2026-04-15", State: json.RawMessage(`{"pk":"abc"}`)},
		},
		{
			name: "version zero coerced to 1",
			in:   ResumeToken{Version: 0, Window: "2026-04-09"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, err := EncodeToken(tc.in)
			if err != nil {
				t.Fatalf("Encode err: %v", err)
			}
			got, err := DecodeToken(s)
			if err != nil {
				t.Fatalf("Decode err: %v", err)
			}
			if got.Version != 1 {
				t.Fatalf("version=%d want 1", got.Version)
			}
			if got.Window != tc.in.Window {
				t.Fatalf("window=%q want %q", got.Window, tc.in.Window)
			}
			if !rawEqual(got.State, tc.in.State) {
				t.Fatalf("state=%q want %q", string(got.State), string(tc.in.State))
			}
		})
	}
}

func TestDecodeToken_Failures(t *testing.T) {
	t.Parallel()
	// 잘못된 base64 / 잘못된 JSON / Version mismatch / Window 형식 오류 / empty.
	validV2, _ := EncodeToken(ResumeToken{Version: 2, Window: "2026-04-09"})
	emptyWindow, _ := EncodeToken(ResumeToken{Version: 1, Window: ""})
	// Window 빈 문자열은 Encode 시 빈으로 직렬화됨. Decode에서 검증해야 함.
	// 만약 위 EncodeToken이 Version==0 coerce를 거치면 빈 윈도우 토큰도 그대로 만들어진다.

	cases := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"invalid base64", "!!!not-base64!!!"},
		{"valid base64 invalid json", "bm90LWpzb24="}, // "not-json"
		{"version 2", validV2},
		{"empty window", emptyWindow},
		{"invalid window format", mustEncode(t, ResumeToken{Version: 1, Window: "2026/04/09"})},
		{"invalid window date", mustEncode(t, ResumeToken{Version: 1, Window: "2026-13-99"})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeToken(tc.token)
			if err == nil {
				t.Fatalf("want err, got nil for token=%q", tc.token)
			}
		})
	}
}

func TestEncodeToken_StableForm(t *testing.T) {
	t.Parallel()
	// 동일 입력은 동일 출력 (deterministic).
	in := ResumeToken{Version: 1, Window: "2026-04-09", State: json.RawMessage(`{"pk":"abc"}`)}
	a, err := EncodeToken(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	b, err := EncodeToken(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if a != b {
		t.Fatalf("encode not deterministic: a=%q b=%q", a, b)
	}
}

// --- helpers -------------------------------------------------------------

func mustEncode(t *testing.T, tok ResumeToken) string {
	t.Helper()
	s, err := EncodeToken(tok)
	if err != nil {
		t.Fatalf("mustEncode failed: %v", err)
	}
	return s
}

func rawEqual(a, b json.RawMessage) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return string(a) == string(b)
}
