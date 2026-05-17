package progress

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/solapi/solactl/pkg/types"
)

// --- Format helpers -------------------------------------------------------

func TestFormatWindowStart(t *testing.T) {
	tests := []struct {
		name        string
		windowIndex int
		total       int
		label       string
		want        string
	}{
		{"first window", 1, 31, "2026-04-08", "다운로드 현황 1/31 진행 중 (2026-04-08)"},
		{"single window", 1, 1, "2026-05-08", "다운로드 현황 1/1 진행 중 (2026-05-08)"},
		{"empty label preserved", 7, 31, "", "다운로드 현황 7/31 진행 중 ()"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatWindowStart(tt.windowIndex, tt.total, tt.label)
			if got != tt.want {
				t.Errorf("FormatWindowStart() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatPageProgress(t *testing.T) {
	tests := []struct {
		name  string
		wIdx  int
		total int
		page  int
		cum   int
		label string
		want  string
	}{
		{
			name: "typical",
			wIdx: 7, total: 31, page: 2, cum: 1250, label: "2026-04-08",
			want: "다운로드 현황 7/31 진행 중 (2026-04-08) | 페이지 2 | 누적 1,250건",
		},
		{
			name: "zero cumulative",
			wIdx: 1, total: 1, page: 1, cum: 0, label: "2026-05-08",
			want: "다운로드 현황 1/1 진행 중 (2026-05-08) | 페이지 1 | 누적 0건",
		},
		{
			name: "millions",
			wIdx: 30, total: 31, page: 99, cum: 1234567, label: "2026-04-30",
			want: "다운로드 현황 30/31 진행 중 (2026-04-30) | 페이지 99 | 누적 1,234,567건",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatPageProgress(tt.wIdx, tt.total, tt.page, tt.cum, tt.label)
			if got != tt.want {
				t.Errorf("FormatPageProgress() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatWindowDone(t *testing.T) {
	got := FormatWindowDone(7, 31, "2026-04-08", 1250)
	want := "다운로드 현황 7/31 완료 (2026-04-08, 누적 1,250건)"
	if got != want {
		t.Errorf("FormatWindowDone() = %q, want %q", got, want)
	}
}

func TestFormatFinalize_Success(t *testing.T) {
	got := FormatFinalize(nil, 31, 1250, time.Minute+2*time.Second)
	want := "전체 다운로드 완료. 총 31/31 윈도우, 누적 1,250건, 소요 시간 1m2s."
	if got != want {
		t.Errorf("FormatFinalize success = %q, want %q", got, want)
	}
}

func TestFormatFinalize_Canceled(t *testing.T) {
	got := FormatFinalize(context.Canceled, 31, 1250, time.Minute+2*time.Second)
	want := "중단됨. 누적 1,250건 처리, 소요 시간 1m2s."
	if got != want {
		t.Errorf("FormatFinalize canceled = %q, want %q", got, want)
	}
}

func TestFormatFinalize_Canceled_Wrapped(t *testing.T) {
	// errors.Is should unwrap to context.Canceled.
	wrapped := fmt.Errorf("aborted: %w", context.Canceled)
	got := FormatFinalize(wrapped, 5, 100, 5*time.Second)
	want := "중단됨. 누적 100건 처리, 소요 시간 5s."
	if got != want {
		t.Errorf("FormatFinalize wrapped-canceled = %q, want %q", got, want)
	}
}

func TestFormatFinalize_Error(t *testing.T) {
	got := FormatFinalize(errors.New("boom"), 5, 100, 5*time.Second)
	want := "오류로 종료. 누적 100건 처리, 마지막 오류: boom"
	if got != want {
		t.Errorf("FormatFinalize error = %q, want %q", got, want)
	}
}

// TestFormatThousands_DelegatesToTypes는 wrapper가 본체에 그대로 위임하는지만
// 검증한다. 본체 함수의 boundary/sign/edge case는 pkg/types/numbers_test.go가
// 책임지므로 동일 보일러플레이트를 두 패키지에 두지 않는다 (drift 방지).
func TestFormatThousands_DelegatesToTypes(t *testing.T) {
	samples := []int{0, 1, 999, 1000, -1234567}
	for _, n := range samples {
		if got, want := formatThousands(n), types.FormatThousands(n); got != want {
			t.Errorf("formatThousands(%d) = %q, types.FormatThousands = %q", n, got, want)
		}
	}
}

// --- Reporter behavior ----------------------------------------------------

// fakeClock returns sequential timestamps controlled by advancing `now`.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func TestReporter_ModeOff_NoOutput(t *testing.T) {
	var buf bytes.Buffer
	r := New(Options{Writer: &buf, Mode: ModeOff})
	r.WindowStart(1, 1, "2026-04-08")
	r.PageProgress(1, 1, 1, 100)
	r.WindowDone(1, 1, "2026-04-08", 100)
	r.Finalize(nil, 100, time.Second)
	if buf.Len() != 0 {
		t.Errorf("expected no output in ModeOff, got: %q", buf.String())
	}
}

func TestReporter_NilWriter_NoPanic(t *testing.T) {
	// Defensive: Options.Writer=nil should not panic.
	r := New(Options{Writer: nil, Mode: ModeOn})
	r.WindowStart(1, 1, "x")
	r.PageProgress(1, 1, 1, 1)
	r.WindowDone(1, 1, "x", 1)
	r.Finalize(nil, 1, time.Second)
}

func TestReporter_ModeOn_FlushIntervalCoalescesPages(t *testing.T) {
	var buf bytes.Buffer
	clk := newFakeClock()
	r := New(Options{
		Writer:        &buf,
		Mode:          ModeOn,
		FlushInterval: 100 * time.Millisecond,
		Now:           clk.Now,
	})

	r.WindowStart(1, 2, "2026-04-08")
	startOut := buf.String()
	if startOut == "" {
		t.Fatal("WindowStart produced no output")
	}

	// First PageProgress fires (lastWrite == WindowStart's time but FlushInterval
	// starts the clock; check: PageProgress only gates if non-zero elapsed < flush)
	// Advance just enough so the first page emits, then call two more in rapid
	// succession.
	clk.Advance(101 * time.Millisecond)
	r.PageProgress(1, 2, 1, 100)
	afterPage1 := buf.String()
	if afterPage1 == startOut {
		t.Fatal("first PageProgress after FlushInterval should emit output")
	}

	// Within FlushInterval: should be coalesced (no new output).
	clk.Advance(50 * time.Millisecond)
	r.PageProgress(1, 2, 2, 200)
	if buf.String() != afterPage1 {
		t.Errorf("PageProgress within FlushInterval should be coalesced; got new output: %q", buf.String()[len(afterPage1):])
	}

	// Past FlushInterval again: should emit.
	clk.Advance(101 * time.Millisecond)
	r.PageProgress(1, 2, 3, 300)
	if buf.String() == afterPage1 {
		t.Errorf("PageProgress after FlushInterval expired should emit, but no new output")
	}
}

func TestReporter_LifecycleEventsBypassFlushInterval(t *testing.T) {
	var buf bytes.Buffer
	clk := newFakeClock()
	r := New(Options{
		Writer:        &buf,
		Mode:          ModeOn,
		FlushInterval: 1 * time.Hour, // huge — would block everything if applied
		Now:           clk.Now,
	})

	r.WindowStart(1, 2, "2026-04-08")
	out1 := buf.String()
	if out1 == "" {
		t.Fatal("WindowStart should emit immediately")
	}

	// Even without advancing the clock, lifecycle events bypass FlushInterval.
	r.WindowDone(1, 2, "2026-04-08", 100)
	out2 := buf.String()
	if out2 == out1 {
		t.Error("WindowDone should bypass FlushInterval")
	}

	r.WindowStart(2, 2, "2026-04-09")
	out3 := buf.String()
	if out3 == out2 {
		t.Error("Second WindowStart should bypass FlushInterval")
	}

	r.Finalize(nil, 200, 5*time.Second)
	out4 := buf.String()
	if out4 == out3 {
		t.Error("Finalize should bypass FlushInterval")
	}
}

func TestReporter_ModeOn_TTYFormat(t *testing.T) {
	var buf bytes.Buffer
	clk := newFakeClock()
	r := New(Options{Writer: &buf, Mode: ModeOn, FlushInterval: 1 * time.Millisecond, Now: clk.Now})

	r.WindowStart(1, 31, "2026-04-08")
	s := buf.String()
	if !strings.HasPrefix(s, "\r") {
		t.Errorf("TTY WindowStart should start with \\r, got: %q", s)
	}
	if strings.HasSuffix(s, "\n") {
		t.Errorf("TTY WindowStart should NOT end with \\n (in-place update), got: %q", s)
	}

	buf.Reset()
	clk.Advance(2 * time.Millisecond)
	r.PageProgress(1, 31, 1, 100)
	s = buf.String()
	if !strings.HasPrefix(s, "\r") {
		t.Errorf("TTY PageProgress should start with \\r, got: %q", s)
	}
	if strings.HasSuffix(s, "\n") {
		t.Errorf("TTY PageProgress should NOT end with \\n, got: %q", s)
	}

	buf.Reset()
	r.WindowDone(1, 31, "2026-04-08", 100)
	s = buf.String()
	if !strings.HasPrefix(s, "\r") {
		t.Errorf("TTY WindowDone should start with \\r, got: %q", s)
	}
	if !strings.HasSuffix(s, "\n") {
		t.Errorf("TTY WindowDone should end with \\n (line terminator), got: %q", s)
	}

	buf.Reset()
	r.Finalize(nil, 100, time.Second)
	s = buf.String()
	if !strings.HasSuffix(s, "\n") {
		t.Errorf("TTY Finalize should end with \\n, got: %q", s)
	}
}

func TestReporter_ModeOn_PadsLongerPreviousLine(t *testing.T) {
	// Two non-terminating writes in sequence, second shorter: verify the second
	// pads with spaces to erase the leftover tail from the first.
	var buf bytes.Buffer
	clk := newFakeClock()
	r := New(Options{Writer: &buf, Mode: ModeOn, FlushInterval: 1 * time.Millisecond, Now: clk.Now})

	r.WindowStart(1, 31, "2026-04-08")
	clk.Advance(2 * time.Millisecond)
	// Long page line (large cumulative): becomes the "previous" line.
	r.PageProgress(1, 31, 99, 9_999_999)
	longMsg := FormatPageProgress(1, 31, 99, 9_999_999, "2026-04-08")
	longLen := runeCount(longMsg)

	buf.Reset()
	clk.Advance(2 * time.Millisecond)
	// Short page line (cumulative=0, single-digit page) overwrites.
	r.PageProgress(1, 31, 1, 0)
	shortMsg := FormatPageProgress(1, 31, 1, 0, "2026-04-08")
	shortLen := runeCount(shortMsg)
	if shortLen >= longLen {
		t.Fatalf("test setup error: shortLen=%d should be < longLen=%d", shortLen, longLen)
	}
	wantPad := strings.Repeat(" ", longLen-shortLen)
	got := buf.String()
	if !strings.HasPrefix(got, "\r"+shortMsg+wantPad) {
		t.Errorf("expected \\r + msg + %d padding spaces, got: %q", longLen-shortLen, got)
	}
	if strings.HasSuffix(got, "\n") {
		t.Errorf("padded line must not end with \\n, got: %q", got)
	}
}

func TestReporter_NonTTY_Auto_WhenWriterIsBuffer(t *testing.T) {
	var buf bytes.Buffer
	clk := newFakeClock()
	r := New(Options{Writer: &buf, Mode: ModeAuto, FlushInterval: 1 * time.Millisecond, Now: clk.Now})

	r.WindowStart(1, 2, "2026-04-08")
	clk.Advance(2 * time.Millisecond)
	r.PageProgress(1, 2, 1, 100)
	clk.Advance(2 * time.Millisecond)
	r.WindowDone(1, 2, "2026-04-08", 100)
	r.Finalize(nil, 100, time.Second)

	out := buf.String()
	if strings.Contains(out, "\r") {
		t.Errorf("non-TTY output should not contain \\r, got: %q", out)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines (start/page/done/finalize), got %d: %q", len(lines), out)
	}
	for i, line := range lines {
		// Timestamp prefix "2026-..." followed by a space.
		if !strings.HasPrefix(line, "2026-") {
			t.Errorf("line %d missing timestamp prefix: %q", i, line)
		}
	}
}

func TestReporter_NonTTY_PageProgress_AlsoCoalesced(t *testing.T) {
	var buf bytes.Buffer
	clk := newFakeClock()
	r := New(Options{Writer: &buf, Mode: ModeAuto, FlushInterval: 100 * time.Millisecond, Now: clk.Now})

	r.WindowStart(1, 1, "x")
	clk.Advance(150 * time.Millisecond)
	r.PageProgress(1, 1, 1, 10)
	first := buf.String()

	// Within flush: should be skipped.
	clk.Advance(20 * time.Millisecond)
	r.PageProgress(1, 1, 2, 20)
	if buf.String() != first {
		t.Errorf("non-TTY PageProgress within FlushInterval should be coalesced; new output: %q", buf.String()[len(first):])
	}

	clk.Advance(200 * time.Millisecond)
	r.PageProgress(1, 1, 3, 30)
	if buf.String() == first {
		t.Error("non-TTY PageProgress past FlushInterval should emit")
	}
}

func TestReporter_PageProgress_BeforeWindowStart_UsesEmptyLabel(t *testing.T) {
	// Defensive: caller should always invoke WindowStart first, but if not,
	// PageProgress must not panic; it just uses "" as the label.
	var buf bytes.Buffer
	clk := newFakeClock()
	r := New(Options{Writer: &buf, Mode: ModeAuto, FlushInterval: 1 * time.Millisecond, Now: clk.Now})

	r.PageProgress(1, 1, 1, 100)
	out := buf.String()
	if !strings.Contains(out, "다운로드 현황 1/1 진행 중 ()") {
		t.Errorf("expected empty label rendering, got: %q", out)
	}
}

func TestReporter_Finalize_TerminatesWithNewline(t *testing.T) {
	tests := []struct {
		name string
		mode Mode
	}{
		{"TTY", ModeOn},
		{"non-TTY", ModeAuto},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			clk := newFakeClock()
			r := New(Options{Writer: &buf, Mode: tt.mode, FlushInterval: 1 * time.Millisecond, Now: clk.Now})

			r.WindowStart(1, 1, "x")
			clk.Advance(2 * time.Millisecond)
			r.PageProgress(1, 1, 1, 100)
			r.Finalize(nil, 100, time.Second)

			if !strings.HasSuffix(buf.String(), "\n") {
				t.Errorf("output must terminate with \\n, got: %q", buf.String())
			}
		})
	}
}

func TestReporter_Finalize_ErrorMessage_PropagatesToOutput(t *testing.T) {
	var buf bytes.Buffer
	clk := newFakeClock()
	r := New(Options{Writer: &buf, Mode: ModeAuto, FlushInterval: 1 * time.Millisecond, Now: clk.Now})

	r.WindowStart(1, 2, "x")
	r.Finalize(errors.New("network down"), 42, 3*time.Second)
	out := buf.String()
	if !strings.Contains(out, "오류로 종료. 누적 42건 처리, 마지막 오류: network down") {
		t.Errorf("expected error summary in output, got: %q", out)
	}
}

func TestReporter_Finalize_Canceled_PropagatesToOutput(t *testing.T) {
	var buf bytes.Buffer
	clk := newFakeClock()
	r := New(Options{Writer: &buf, Mode: ModeAuto, FlushInterval: 1 * time.Millisecond, Now: clk.Now})

	r.WindowStart(1, 2, "x")
	r.Finalize(context.Canceled, 7, 250*time.Millisecond)
	out := buf.String()
	if !strings.Contains(out, "중단됨. 누적 7건 처리, 소요 시간 250ms.") {
		t.Errorf("expected cancellation summary, got: %q", out)
	}
}

func TestReporter_Concurrent_NoRace(t *testing.T) {
	// Spec says single-goroutine, but we enforce mutex anyway. With -race the
	// detector will catch any mistake.
	var buf bytes.Buffer
	clk := newFakeClock()
	r := New(Options{Writer: &buf, Mode: ModeOn, FlushInterval: 1 * time.Millisecond, Now: clk.Now})

	r.WindowStart(1, 1, "x")

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	var counter int64
	for i := range goroutines {
		go func(n int) {
			defer wg.Done()
			clk.Advance(2 * time.Millisecond)
			r.PageProgress(1, 1, n, int(atomic.AddInt64(&counter, 1)))
		}(i)
	}
	wg.Wait()

	r.Finalize(nil, int(atomic.LoadInt64(&counter)), 10*time.Millisecond)

	// Sanity: output is non-empty and terminates with \n.
	if buf.Len() == 0 {
		t.Error("expected some output from concurrent calls")
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Errorf("expected trailing \\n, got: %q", buf.String())
	}
}

// --- TTY detection --------------------------------------------------------

func TestIsTTY_BufferIsNotTTY(t *testing.T) {
	var buf bytes.Buffer
	if isTTY(&buf) {
		t.Error("bytes.Buffer should not be considered a TTY")
	}
}

func TestIsTTY_RegularFileIsNotTTY(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "progress-tty-*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	if isTTY(f) {
		t.Error("regular file should not be considered a TTY")
	}
}

func TestIsTTY_ClosedFile_IsNotTTY(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "progress-closed-*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	_ = f.Close()

	// Stat may or may not fail on a closed *os.File depending on platform,
	// but in either case isTTY must not panic and must return false for a
	// regular (non-char-device) file.
	if isTTY(f) {
		t.Error("closed regular file should never be a TTY")
	}
}

// --- noopReporter direct tests --------------------------------------------

func TestNoopReporter_NeverWrites(t *testing.T) {
	// Ensure noopReporter is what New(ModeOff) returns and exercises every method.
	var r Reporter = noopReporter{}
	r.WindowStart(1, 1, "x")
	r.PageProgress(1, 1, 1, 1)
	r.WindowDone(1, 1, "x", 1)
	r.Finalize(errors.New("x"), 1, time.Second)
	// Reaching here without panic is the assertion.
}

// --- compile-time assertion that *reporter satisfies Reporter -------------

var _ Reporter = (*reporter)(nil)
var _ Reporter = noopReporter{}

// Sanity: io.Writer is the type used in Options; ensure import is used.
var _ io.Writer = (*bytes.Buffer)(nil)
