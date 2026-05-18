// Package progress reports export progress to a writer (typically stderr).
// It supports TTY (\r-overwrite) and non-TTY (timestamped append) styles, with
// FlushInterval coalescing for high-frequency page events.
package progress

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/solapi/solactl/pkg/types"
)

// Reporter is the abstraction exporters depend on.
type Reporter interface {
	WindowStart(windowIndex, totalWindows int, windowLabel string)
	PageProgress(windowIndex, totalWindows, pageIndex int, cumulativeRecords int)
	WindowDone(windowIndex, totalWindows int, windowLabel string, cumulativeRecords int)
	Finalize(err error, cumulativeRecords int, elapsed time.Duration)
}

// Mode selects the rendering style.
type Mode int

const (
	// ModeAuto detects TTY from the writer.
	ModeAuto Mode = iota
	// ModeOn forces TTY-style (\r refresh).
	ModeOn
	// ModeOff disables all output.
	ModeOff
)

// Options configures a Reporter.
type Options struct {
	Writer        io.Writer
	Mode          Mode
	FlushInterval time.Duration
	Now           func() time.Time
}

const defaultFlushInterval = 100 * time.Millisecond

// New creates a Reporter from Options. ModeOff returns a no-op reporter.
// Writer that is not an *os.File character device is treated as non-TTY.
func New(opts Options) Reporter {
	if opts.Mode == ModeOff || opts.Writer == nil {
		return noopReporter{}
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	flush := opts.FlushInterval
	if flush <= 0 {
		flush = defaultFlushInterval
	}
	tty := opts.Mode == ModeOn || (opts.Mode == ModeAuto && isTTY(opts.Writer))

	return &reporter{
		w:         opts.Writer,
		tty:       tty,
		flush:     flush,
		now:       now,
		totalWins: 0,
	}
}

// FormatWindowStart renders the start-of-window message.
func FormatWindowStart(windowIndex, totalWindows int, windowLabel string) string {
	return fmt.Sprintf("다운로드 현황 %d/%d 진행 중 (%s)", windowIndex, totalWindows, windowLabel)
}

// FormatPageProgress renders the per-page progress message.
func FormatPageProgress(windowIndex, totalWindows, pageIndex, cumulativeRecords int, windowLabel string) string {
	return fmt.Sprintf("다운로드 현황 %d/%d 진행 중 (%s) | 페이지 %d | 누적 %s건",
		windowIndex, totalWindows, windowLabel, pageIndex, formatThousands(cumulativeRecords))
}

// FormatWindowDone renders the end-of-window message.
func FormatWindowDone(windowIndex, totalWindows int, windowLabel string, cumulativeRecords int) string {
	return fmt.Sprintf("다운로드 현황 %d/%d 완료 (%s, 누적 %s건)",
		windowIndex, totalWindows, windowLabel, formatThousands(cumulativeRecords))
}

// FormatFinalize renders the terminal summary. err==nil success; context.Canceled cancel; other err failure.
func FormatFinalize(err error, totalWindows, cumulativeRecords int, elapsed time.Duration) string {
	switch {
	case err == nil:
		return fmt.Sprintf("전체 다운로드 완료. 총 %d/%d 윈도우, 누적 %s건, 소요 시간 %s.",
			totalWindows, totalWindows, formatThousands(cumulativeRecords), elapsed)
	case errors.Is(err, context.Canceled):
		return fmt.Sprintf("중단됨. 누적 %s건 처리, 소요 시간 %s.",
			formatThousands(cumulativeRecords), elapsed)
	default:
		return fmt.Sprintf("오류로 종료. 누적 %s건 처리, 마지막 오류: %s",
			formatThousands(cumulativeRecords), err.Error())
	}
}

// reporter is the concrete Reporter. Methods serialize via mu so a misuse from
// multiple goroutines stays race-free (the spec assumes single-goroutine use,
// but the race detector demands it anyway).
type reporter struct {
	mu sync.Mutex
	w  io.Writer

	tty   bool
	flush time.Duration
	now   func() time.Time

	totalWins    int
	currentLabel string // remembered from the most recent WindowStart for PageProgress messages

	lastWrite  time.Time
	hasLastLen bool
	lastLen    int // for TTY: length of last \r-written line, for padding the next overwrite
}

func (r *reporter) WindowStart(windowIndex, totalWindows int, windowLabel string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.totalWins = totalWindows
	r.currentLabel = windowLabel
	r.writeLine(FormatWindowStart(windowIndex, totalWindows, windowLabel), false)
}

func (r *reporter) PageProgress(windowIndex, totalWindows, pageIndex, cumulativeRecords int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.totalWins == 0 {
		r.totalWins = totalWindows
	}
	// Coalesce high-frequency page events. WindowStart/Done/Finalize bypass this gate.
	if !r.lastWrite.IsZero() && r.now().Sub(r.lastWrite) < r.flush {
		return
	}
	msg := FormatPageProgress(windowIndex, totalWindows, pageIndex, cumulativeRecords, r.currentLabel)
	r.writeLine(msg, false)
}

func (r *reporter) WindowDone(windowIndex, totalWindows int, windowLabel string, cumulativeRecords int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.writeLine(FormatWindowDone(windowIndex, totalWindows, windowLabel, cumulativeRecords), true)
}

func (r *reporter) Finalize(err error, cumulativeRecords int, elapsed time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.writeLine(FormatFinalize(err, r.totalWins, cumulativeRecords, elapsed), true)
}

// writeLine renders msg with TTY (\r refresh) or non-TTY (timestamp + \n) style.
// terminate=true forces a trailing \n (window done / finalize). Callers must
// gate FlushInterval upstream (PageProgress does; lifecycle events bypass).
func (r *reporter) writeLine(msg string, terminate bool) {
	if !r.tty {
		stamp := r.now().UTC().Format("2006-01-02T15:04:05Z")
		_, _ = io.WriteString(r.w, stamp+" "+msg+"\n")
		r.lastWrite = r.now()
		return
	}

	// TTY: pad with spaces to erase any leftover characters from the previous
	// (potentially longer) line, then \r so the next write overwrites again.
	var b strings.Builder
	b.WriteByte('\r')
	b.WriteString(msg)
	if r.hasLastLen && r.lastLen > runeCount(msg) {
		b.WriteString(strings.Repeat(" ", r.lastLen-runeCount(msg)))
	}
	if terminate {
		b.WriteByte('\n')
		r.hasLastLen = false
		r.lastLen = 0
	} else {
		r.hasLastLen = true
		r.lastLen = runeCount(msg)
	}
	_, _ = io.WriteString(r.w, b.String())
	r.lastWrite = r.now()
}

// noopReporter satisfies Reporter for ModeOff.
type noopReporter struct{}

func (noopReporter) WindowStart(int, int, string)       {}
func (noopReporter) PageProgress(int, int, int, int)    {}
func (noopReporter) WindowDone(int, int, string, int)   {}
func (noopReporter) Finalize(error, int, time.Duration) {}

// isTTY checks whether w is a character device. Anything other than *os.File
// (e.g. *bytes.Buffer) returns false so tests are deterministic.
func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// formatThousands wraps types.FormatThousands so we don't duplicate the logic.
func formatThousands(n int) string { return types.FormatThousands(n) }

// runeCount counts runes, not bytes — Korean characters occupy multiple bytes
// but render as a single display column for our padding heuristic. This is a
// reasonable approximation: terminals using East-Asian wide widths may still
// see minor padding artifacts, which is acceptable for a progress line that
// gets overwritten anyway.
func runeCount(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
