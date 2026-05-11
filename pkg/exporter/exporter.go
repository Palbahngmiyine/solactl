package exporter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/solapi/solactl/pkg/clock"
	"github.com/solapi/solactl/pkg/progress"
)

// Defaults applied when the corresponding Options field is zero/empty.
const (
	DefaultMaxLookbackDays = 180
	DefaultMaxWindowDays   = 7
	DefaultPageSize        = 50
	DefaultThrottle        = 500 * time.Millisecond
	MinThrottle            = 100 * time.Millisecond
)

// PageState is per-fetcher opaque cursor state. nil = start of window.
type PageState = json.RawMessage

// Page is one fetched page. Next==nil signals end-of-window.
type Page struct {
	Records []json.RawMessage
	Next    PageState
}

// PageFetcher fetches one page within w. state==nil means first page in window.
type PageFetcher func(ctx context.Context, w Window, state PageState) (Page, error)

// RowWriter accepts raw JSON records. Implementations are not required to be
// goroutine-safe — Run serializes all calls.
type RowWriter interface {
	WriteRecord(record json.RawMessage) error
	Flush() error
}

// Options configures Run. Required fields: Now, StartDate, EndDate, Fetcher, Writer.
type Options struct {
	Now             time.Time
	StartDate       time.Time
	EndDate         time.Time
	PageSize        int
	MaxPages        int
	Throttle        time.Duration
	MaxLookbackDays int
	MaxWindowDays   int
	Fetcher         PageFetcher
	Writer          RowWriter
	Reporter        progress.Reporter
	Clock           clock.Clock

	// Resume from a specific window. Empty = start from the first window.
	StartWindowDate string
	// State for the first page of the resumed window. Used only on the first
	// iteration when StartWindowDate matches.
	InitialState PageState
}

// Result is returned by Run. ResumeToken!="" indicates a recoverable interruption.
type Result struct {
	RecordsWritten int
	ResumeToken    string
}

// ValidatePageSize fails if value > hardMax. value<=0 means "use default" and
// is left for the caller to fill in; the validator only rejects over-limit.
func ValidatePageSize(name string, value, hardMax int) error {
	if hardMax <= 0 {
		return fmt.Errorf("%s: hardMax must be positive, got %d", name, hardMax)
	}
	if value > hardMax {
		return fmt.Errorf("%s: %d exceeds max %d", name, value, hardMax)
	}
	return nil
}

// ValidateThrottle fails if d < minD. minD<=0 → MinThrottle.
// d<=0 is treated as "use default" by Run, so we permit it here as well.
func ValidateThrottle(d, minD time.Duration) error {
	if minD <= 0 {
		minD = MinThrottle
	}
	if d <= 0 {
		return nil
	}
	if d < minD {
		return fmt.Errorf("throttle %s below minimum %s", d, minD)
	}
	return nil
}

// Run drives the windowed pagination loop. See package docs and Options
// fields for semantics. Reporter.Finalize is always called (except on panic).
func Run(ctx context.Context, opts Options) (Result, error) {
	if opts.Fetcher == nil {
		return Result{}, errors.New("exporter: Fetcher is required")
	}
	if opts.Writer == nil {
		return Result{}, errors.New("exporter: Writer is required")
	}

	// Default-fill before any validation that depends on these values.
	if opts.MaxLookbackDays <= 0 {
		opts.MaxLookbackDays = DefaultMaxLookbackDays
	}
	if opts.MaxWindowDays <= 0 {
		opts.MaxWindowDays = DefaultMaxWindowDays
	}
	if opts.Throttle <= 0 {
		opts.Throttle = DefaultThrottle
	}
	if opts.Clock == nil {
		opts.Clock = clock.Real()
	}
	if opts.Reporter == nil {
		opts.Reporter = progress.New(progress.Options{Mode: progress.ModeOff})
	}

	// Validation (post-default). Note: PageSize<=0 is *allowed* — caller may rely
	// on server-side default. ValidatePageSize is a callable helper for callers.
	if err := ValidateThrottle(opts.Throttle, MinThrottle); err != nil {
		return Result{}, err
	}

	effectiveEnd, err := ValidateDateRange(opts.StartDate, opts.EndDate, opts.Now, opts.MaxLookbackDays)
	if err != nil {
		return Result{}, err
	}

	windows := SplitWindows(opts.StartDate, effectiveEnd, opts.MaxWindowDays)
	if len(windows) == 0 {
		// 검증을 통과했는데 윈도우가 비어 있으면 내부 invariant 위반.
		return Result{}, errors.New("exporter: no windows after split (internal invariant)")
	}

	// Resume: jump to the window with matching Label().
	skipIdx := 0
	if opts.StartWindowDate != "" {
		found := -1
		for i, w := range windows {
			if w.Label() == opts.StartWindowDate {
				found = i
				break
			}
		}
		if found < 0 {
			return Result{}, fmt.Errorf("exporter: resume window %q not in split range", opts.StartWindowDate)
		}
		skipIdx = found
	}

	totalWindows := len(windows)
	startTime := opts.Clock.Now()

	var (
		recordsWritten int
		totalPages     int
		runErr         error
		// resumeWindowIdx / resumeState describe the window/state to resume from.
		// resumeWindowIdx==-1 ⇒ no resume token (successful completion).
		resumeWindowIdx = -1
		resumeState     PageState
	)

	// finalize는 모든 종료 경로에서 정확히 1회 호출되도록 함수 끝에서 deferred.
	finalize := func() Result {
		elapsed := opts.Clock.Now().Sub(startTime)
		opts.Reporter.Finalize(runErr, recordsWritten, elapsed)
		res := Result{RecordsWritten: recordsWritten}
		if resumeWindowIdx >= 0 && resumeWindowIdx < totalWindows {
			tok, encErr := EncodeToken(ResumeToken{
				Version: resumeTokenVersion,
				Window:  windows[resumeWindowIdx].Label(),
				State:   resumeState,
			})
			if encErr == nil {
				res.ResumeToken = tok
			} else if runErr == nil {
				runErr = encErr
			}
		}
		return res
	}

windowLoop:
	for i := skipIdx; i < totalWindows; i++ {
		w := windows[i]

		// 사이클 진입 전 cancel 점검.
		if cerr := ctx.Err(); cerr != nil {
			runErr = cerr
			resumeWindowIdx = i
			resumeState = nil
			break
		}

		opts.Reporter.WindowStart(i+1, totalWindows, w.Label())

		var state PageState
		if i == skipIdx {
			state = opts.InitialState
		}

		pageIdx := 0
		for {
			if cerr := ctx.Err(); cerr != nil {
				runErr = cerr
				resumeWindowIdx = i
				resumeState = state
				break windowLoop
			}

			page, ferr := opts.Fetcher(ctx, w, state)
			if ferr != nil {
				runErr = ferr
				resumeWindowIdx = i
				resumeState = state
				break windowLoop
			}

			for _, rec := range page.Records {
				if werr := opts.Writer.WriteRecord(rec); werr != nil {
					runErr = werr
					resumeWindowIdx = i
					resumeState = state
					break windowLoop
				}
				recordsWritten++
			}
			if ferr := opts.Writer.Flush(); ferr != nil {
				runErr = ferr
				resumeWindowIdx = i
				resumeState = state
				break windowLoop
			}

			pageIdx++
			totalPages++
			opts.Reporter.PageProgress(i+1, totalWindows, pageIdx, recordsWritten)

			// MaxPages 도달: 다음 페이지를 위해 token 인코딩. 같은 윈도우 내 next state 사용.
			if opts.MaxPages > 0 && totalPages >= opts.MaxPages {
				if len(page.Next) > 0 {
					resumeWindowIdx = i
					resumeState = page.Next
				} else if i+1 < totalWindows {
					// 현재 윈도우는 끝났고 다음 윈도우부터 재개.
					resumeWindowIdx = i + 1
					resumeState = nil
				}
				break windowLoop
			}

			if len(page.Next) == 0 {
				// 윈도우 종료. nil 또는 빈 RawMessage 모두 종료 신호로 처리해 무한 루프 회피.
				break
			}
			state = page.Next

			// 페이지 사이 throttle. ctx cancel이면 escape.
			if serr := opts.Clock.Sleep(ctx, opts.Throttle); serr != nil {
				runErr = serr
				resumeWindowIdx = i
				resumeState = state
				break windowLoop
			}
		}

		opts.Reporter.WindowDone(i+1, totalWindows, w.Label(), recordsWritten)

		// 윈도우 사이 throttle (마지막 윈도우 후에는 sleep 없음).
		if i+1 < totalWindows {
			if serr := opts.Clock.Sleep(ctx, opts.Throttle); serr != nil {
				runErr = serr
				resumeWindowIdx = i + 1
				resumeState = nil
				break
			}
		}
	}

	return finalize(), runErr
}
