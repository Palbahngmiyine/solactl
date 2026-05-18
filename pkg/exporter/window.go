// Package exporter provides time-windowed paginated export with deterministic
// throttling, progress reporting, and resume-token-based recovery.
package exporter

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Window represents a half-open UTC time window [Start, End).
type Window struct {
	Start, End time.Time
}

// labelLayout is the canonical day-window label format (and resume-token date format).
const labelLayout = "2006-01-02"

// Label returns the "YYYY-MM-DD" form of Start (UTC). Used as window identifier
// in resume tokens and progress messages.
func (w Window) Label() string {
	return w.Start.UTC().Format(labelLayout)
}

// Sentinel errors callers can check with errors.Is.
var (
	// ErrLookbackExceeded reports that the requested start date is older than
	// the configured max-lookback window.
	ErrLookbackExceeded = errors.New("start date exceeds max lookback")
	// ErrStartAfterEnd reports start >= end (empty or inverted range).
	ErrStartAfterEnd = errors.New("start date must be before end date")
)

// floorDayUTC returns the UTC-midnight at the start of t's UTC day.
// time.Truncate(24h) is epoch-relative, not calendar-aware, so we cannot use it.
func floorDayUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// ceilDayUTC returns t if t is already at UTC midnight, otherwise the next UTC midnight.
func ceilDayUTC(t time.Time) time.Time {
	t = t.UTC()
	floor := floorDayUTC(t)
	if t.Equal(floor) {
		return floor
	}
	return floor.Add(24 * time.Hour)
}

// SplitWindows splits [start, end) into UTC-aligned windows.
//
// 규칙 요약:
//   - start >= end → 빈 슬라이스 (호출자 사전 검증 책임)
//   - (end - start) <= maxWindowDays days → 단일 윈도우 [floor(start), ceil(end))
//   - 초과 → 1일짜리 윈도우 N개. N = ceil((ceilEnd - floorStart) / 24h)
//   - maxWindowDays <= 0 → DefaultMaxWindowDays (7) 사용
func SplitWindows(start, end time.Time, maxWindowDays int) []Window {
	if !start.Before(end) {
		return nil
	}
	if maxWindowDays <= 0 {
		maxWindowDays = DefaultMaxWindowDays
	}

	// 단일/분할 결정은 원본(raw) 길이로 한다. 자정 정렬 ceil로 인한 길이 증가는
	// 분할 임계점을 좌우하지 않는다.
	limit := time.Duration(maxWindowDays) * 24 * time.Hour
	startFloor := floorDayUTC(start)
	endCeil := ceilDayUTC(end)

	if end.Sub(start) <= limit {
		return []Window{{Start: startFloor, End: endCeil}}
	}

	// 분할 경로: 정렬된 [startFloor, endCeil) 구간을 24h 단위로 나눈다.
	totalDays := int(endCeil.Sub(startFloor) / (24 * time.Hour))
	if totalDays <= 0 {
		// 방어적: 같은 자정으로 정렬됐다면 단일 윈도우 반환.
		return []Window{{Start: startFloor, End: endCeil}}
	}
	out := make([]Window, 0, totalDays)
	for i := range totalDays {
		s := startFloor.Add(time.Duration(i) * 24 * time.Hour)
		e := s.Add(24 * time.Hour)
		out = append(out, Window{Start: s, End: e})
	}
	return out
}

// ValidateDateRange enforces lookback / ordering / future-clamp policies.
//
//   - start이 (now - maxLookbackDays)보다 이전 → ErrLookbackExceeded.
//   - start >= end → ErrStartAfterEnd.
//   - end > now → effectiveEnd = now, err = nil. (호출자가 effectiveEnd != end로 clamp 인지)
//
// 검증 순서: lookback → start>=end → end>now (clamp).
//
// maxLookbackDays <= 0 → DefaultMaxLookbackDays.
func ValidateDateRange(start, end, now time.Time, maxLookbackDays int) (time.Time, error) {
	if maxLookbackDays <= 0 {
		maxLookbackDays = DefaultMaxLookbackDays
	}
	cutoff := now.Add(-time.Duration(maxLookbackDays) * 24 * time.Hour)
	if start.Before(cutoff) {
		return time.Time{}, fmt.Errorf("%w: 조회 가능한 가장 오래된 날짜: %s",
			ErrLookbackExceeded, cutoff.UTC().Format(labelLayout))
	}
	if !start.Before(end) {
		return time.Time{}, fmt.Errorf("%w", ErrStartAfterEnd)
	}
	if end.After(now) {
		return now, nil
	}
	return end, nil
}

// resumeTokenVersion is the only currently-supported ResumeToken.Version.
const resumeTokenVersion = 1

// ResumeToken carries the next-window cursor across runs. The JSON form is
// base64-encoded (StdEncoding) for safe transport via CLI args/stdout.
type ResumeToken struct {
	Version int             `json:"v"`
	Window  string          `json:"w"`
	State   json.RawMessage `json:"s,omitempty"`
}

// EncodeToken returns the base64-encoded JSON form. Version==0 is coerced to 1.
func EncodeToken(t ResumeToken) (string, error) {
	if t.Version == 0 {
		t.Version = resumeTokenVersion
	}
	buf, err := json.Marshal(t)
	if err != nil {
		return "", fmt.Errorf("encode resume token: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}

// DecodeToken parses base64+JSON. Empty string, malformed base64/JSON,
// unsupported Version, or malformed Window all yield errors.
func DecodeToken(token string) (ResumeToken, error) {
	if token == "" {
		return ResumeToken{}, errors.New("decode resume token: empty token")
	}
	raw, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return ResumeToken{}, fmt.Errorf("decode resume token: base64: %w", err)
	}
	var t ResumeToken
	if err := json.Unmarshal(raw, &t); err != nil {
		return ResumeToken{}, fmt.Errorf("decode resume token: json: %w", err)
	}
	if t.Version != resumeTokenVersion {
		return ResumeToken{}, fmt.Errorf("decode resume token: unsupported version %d", t.Version)
	}
	if t.Window == "" {
		return ResumeToken{}, errors.New("decode resume token: missing window")
	}
	if _, err := time.Parse(labelLayout, t.Window); err != nil {
		return ResumeToken{}, fmt.Errorf("decode resume token: invalid window %q: %w", t.Window, err)
	}
	return t, nil
}
