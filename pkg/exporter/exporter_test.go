package exporter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/solapi/solactl/pkg/clock"
	"github.com/solapi/solactl/pkg/progress"
)

// --- helpers --------------------------------------------------------------

// recordingWriter captures all records written.
type recordingWriter struct {
	mu       sync.Mutex
	records  []json.RawMessage
	flushed  int
	writeErr error // 설정되면 매 WriteRecord 호출에서 반환
	flushErr error // 설정되면 매 Flush 호출에서 반환
}

func (w *recordingWriter) WriteRecord(rec json.RawMessage) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.writeErr != nil {
		return w.writeErr
	}
	cp := make(json.RawMessage, len(rec))
	copy(cp, rec)
	w.records = append(w.records, cp)
	return nil
}
func (w *recordingWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.flushErr != nil {
		return w.flushErr
	}
	w.flushed++
	return nil
}

// recordingReporter captures all reporter callbacks.
type recordingReporter struct {
	mu             sync.Mutex
	starts         []reporterEvent
	pages          []reporterEvent
	dones          []reporterEvent
	finalizeCalls  []finalizeEvent
}

type reporterEvent struct {
	WindowIndex int
	Total       int
	Label       string
	PageIndex   int
	Cumulative  int
}

type finalizeEvent struct {
	Err        error
	Cumulative int
	Elapsed    time.Duration
}

func (r *recordingReporter) WindowStart(wi, total int, label string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.starts = append(r.starts, reporterEvent{WindowIndex: wi, Total: total, Label: label})
}
func (r *recordingReporter) PageProgress(wi, total, pi, cum int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pages = append(r.pages, reporterEvent{WindowIndex: wi, Total: total, PageIndex: pi, Cumulative: cum})
}
func (r *recordingReporter) WindowDone(wi, total int, label string, cum int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dones = append(r.dones, reporterEvent{WindowIndex: wi, Total: total, Label: label, Cumulative: cum})
}
func (r *recordingReporter) Finalize(err error, cum int, elapsed time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.finalizeCalls = append(r.finalizeCalls, finalizeEvent{Err: err, Cumulative: cum, Elapsed: elapsed})
}

// fakeFetcher returns canned pages per (window-label, state-key) pair.
//
// pages 키: "<label>:<stateKey>" — stateKey는 state JSON 그대로 (nil → "").
type fakeFetcher struct {
	mu     sync.Mutex
	pages  map[string]Page
	err    error
	errAt  string // 에러를 발생시킬 키 ("" → 항상)
	cancel context.CancelFunc
	// ctxCancelOnHit: 이 key에 도달하면 ctx를 cancel하고 ctx.Err() 반환.
	ctxCancelOnHit string
	// calls는 호출 시퀀스(key)를 기록.
	calls []string
}

func newFakeFetcher() *fakeFetcher {
	return &fakeFetcher{pages: make(map[string]Page)}
}

func (f *fakeFetcher) set(label string, state PageState, page Page) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pages[fmt.Sprintf("%s:%s", label, string(state))] = page
}

func (f *fakeFetcher) Fetch(ctx context.Context, w Window, state PageState) (Page, error) {
	f.mu.Lock()
	key := fmt.Sprintf("%s:%s", w.Label(), string(state))
	f.calls = append(f.calls, key)
	if f.ctxCancelOnHit != "" && key == f.ctxCancelOnHit && f.cancel != nil {
		f.cancel()
		f.mu.Unlock()
		// cancel은 비동기 — 즉시 ctx.Err 반환으로 시뮬레이션.
		return Page{}, ctx.Err()
	}
	if f.err != nil && (f.errAt == "" || f.errAt == key) {
		err := f.err
		f.mu.Unlock()
		return Page{}, err
	}
	page, ok := f.pages[key]
	f.mu.Unlock()
	if !ok {
		return Page{}, fmt.Errorf("fakeFetcher: no page for key=%q", key)
	}
	return page, nil
}

func raw(s string) json.RawMessage { return json.RawMessage(s) }

// stdOpts: 7일 단일 윈도우 기본 옵션. fetcher / writer / reporter / clock은 호출자가 채움.
func stdOpts(now time.Time) Options {
	return Options{
		Now:             now,
		StartDate:       now.AddDate(0, 0, -3),
		EndDate:         now,
		PageSize:        50,
		Throttle:        500 * time.Millisecond,
		MaxLookbackDays: 180,
		MaxWindowDays:   7,
	}
}

func newFake(now time.Time) (*clock.Fake, *recordingReporter, *recordingWriter) {
	return clock.NewFake(now), &recordingReporter{}, &recordingWriter{}
}

// --- tests ----------------------------------------------------------------

func TestRun_SingleWindowSinglePage(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)
	fk, rep, w := newFake(now)
	fetcher := newFakeFetcher()

	opts := stdOpts(now)
	opts.Fetcher = fetcher.Fetch
	opts.Writer = w
	opts.Reporter = rep
	opts.Clock = fk

	// 단일 윈도우: SplitWindows로 검사하여 label을 미리 알아냄.
	wins := SplitWindows(opts.StartDate, opts.EndDate, opts.MaxWindowDays)
	if len(wins) != 1 {
		t.Fatalf("setup: want 1 window, got %d", len(wins))
	}
	fetcher.set(wins[0].Label(), nil, Page{
		Records: []json.RawMessage{raw(`{"id":1}`), raw(`{"id":2}`), raw(`{"id":3}`)},
		Next:    nil,
	})

	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if res.RecordsWritten != 3 {
		t.Fatalf("records=%d want 3", res.RecordsWritten)
	}
	if res.ResumeToken != "" {
		t.Fatalf("resume token should be empty on success, got %q", res.ResumeToken)
	}
	if len(rep.starts) != 1 || len(rep.dones) != 1 {
		t.Fatalf("reporter starts=%d dones=%d, want 1/1", len(rep.starts), len(rep.dones))
	}
	if len(rep.finalizeCalls) != 1 || rep.finalizeCalls[0].Err != nil {
		t.Fatalf("finalize=%+v", rep.finalizeCalls)
	}
	if len(fk.Sleeps()) != 0 {
		t.Fatalf("single page → no sleeps, got %v", fk.Sleeps())
	}
}

func TestRun_SingleWindowMultiPage(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)
	fk, rep, w := newFake(now)
	fetcher := newFakeFetcher()
	opts := stdOpts(now)
	opts.Fetcher = fetcher.Fetch
	opts.Writer = w
	opts.Reporter = rep
	opts.Clock = fk

	wins := SplitWindows(opts.StartDate, opts.EndDate, opts.MaxWindowDays)
	label := wins[0].Label()
	fetcher.set(label, nil, Page{
		Records: []json.RawMessage{raw(`{"id":1}`), raw(`{"id":2}`), raw(`{"id":3}`)},
		Next:    raw(`"a"`),
	})
	fetcher.set(label, raw(`"a"`), Page{
		Records: []json.RawMessage{raw(`{"id":4}`), raw(`{"id":5}`)},
		Next:    nil,
	})

	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if res.RecordsWritten != 5 {
		t.Fatalf("records=%d want 5", res.RecordsWritten)
	}
	// 페이지 사이 1 sleep (마지막 페이지 후 sleep 없음).
	sleeps := fk.Sleeps()
	if len(sleeps) != 1 {
		t.Fatalf("sleeps=%v want exactly 1 (between pages)", sleeps)
	}
	if sleeps[0] != opts.Throttle {
		t.Fatalf("sleep[0]=%v want %v", sleeps[0], opts.Throttle)
	}
}

func TestRun_MultiWindowSplit_OnePageEach(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)
	fk, rep, w := newFake(now)
	fetcher := newFakeFetcher()

	opts := stdOpts(now)
	opts.StartDate = mid(2026, 4, 2)
	opts.EndDate = mid(2026, 5, 3)
	opts.MaxWindowDays = 7
	opts.Fetcher = fetcher.Fetch
	opts.Writer = w
	opts.Reporter = rep
	opts.Clock = fk

	wins := SplitWindows(opts.StartDate, opts.EndDate, opts.MaxWindowDays)
	if len(wins) != 31 {
		t.Fatalf("setup: want 31 windows, got %d", len(wins))
	}
	for _, win := range wins {
		fetcher.set(win.Label(), nil, Page{
			Records: []json.RawMessage{raw(`{"x":1}`)},
			Next:    nil,
		})
	}

	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if res.RecordsWritten != 31 {
		t.Fatalf("records=%d want 31", res.RecordsWritten)
	}
	// 윈도우 사이 30 sleep, 윈도우 내 페이지 사이 0.
	if len(fk.Sleeps()) != 30 {
		t.Fatalf("sleeps=%d want 30", len(fk.Sleeps()))
	}
	if len(rep.starts) != 31 || len(rep.dones) != 31 {
		t.Fatalf("reporter starts=%d dones=%d, want 31/31", len(rep.starts), len(rep.dones))
	}
}

func TestRun_MultiWindowMultiPage(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)
	fk, rep, w := newFake(now)
	fetcher := newFakeFetcher()

	opts := stdOpts(now)
	opts.StartDate = mid(2026, 4, 2)
	opts.EndDate = mid(2026, 5, 3)
	opts.Fetcher = fetcher.Fetch
	opts.Writer = w
	opts.Reporter = rep
	opts.Clock = fk

	wins := SplitWindows(opts.StartDate, opts.EndDate, opts.MaxWindowDays)
	if len(wins) != 31 {
		t.Fatalf("setup: want 31, got %d", len(wins))
	}
	// 각 윈도우 2페이지.
	for _, win := range wins {
		fetcher.set(win.Label(), nil, Page{Records: []json.RawMessage{raw(`{"a":1}`)}, Next: raw(`"n"`)})
		fetcher.set(win.Label(), raw(`"n"`), Page{Records: []json.RawMessage{raw(`{"b":2}`)}, Next: nil})
	}

	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if res.RecordsWritten != 62 {
		t.Fatalf("records=%d want 62", res.RecordsWritten)
	}
	// 윈도우당 1 페이지간 sleep + 윈도우간 30 sleep = 31+30=61.
	if got := len(fk.Sleeps()); got != 61 {
		t.Fatalf("sleeps=%d want 61", got)
	}
}

func TestRun_FetcherError_PreservesPartialResult(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)
	fk, rep, w := newFake(now)
	fetcher := newFakeFetcher()

	opts := stdOpts(now)
	opts.StartDate = mid(2026, 4, 2)
	opts.EndDate = mid(2026, 5, 3)
	opts.Fetcher = fetcher.Fetch
	opts.Writer = w
	opts.Reporter = rep
	opts.Clock = fk

	wins := SplitWindows(opts.StartDate, opts.EndDate, opts.MaxWindowDays)
	// 처음 4개 윈도우는 성공, 5번째에서 fetcher 에러.
	for i, win := range wins {
		if i < 4 {
			fetcher.set(win.Label(), nil, Page{Records: []json.RawMessage{raw(`{"x":1}`)}, Next: nil})
		}
	}
	errSentinel := errors.New("boom")
	fetcher.err = errSentinel
	fetcher.errAt = fmt.Sprintf("%s:", wins[4].Label()) // 5번째 윈도우 첫 페이지

	res, err := Run(context.Background(), opts)
	if !errors.Is(err, errSentinel) {
		t.Fatalf("err=%v want %v", err, errSentinel)
	}
	if res.RecordsWritten != 4 {
		t.Fatalf("records=%d want 4 (4개 윈도우 완료)", res.RecordsWritten)
	}
	if res.ResumeToken == "" {
		t.Fatal("want resume token on error")
	}
	tok, derr := DecodeToken(res.ResumeToken)
	if derr != nil {
		t.Fatalf("decode token: %v", derr)
	}
	// 에러 발생 윈도우(5번째 = index 4 = label) 부터 재개.
	if tok.Window != wins[4].Label() {
		t.Fatalf("token window=%q want %q", tok.Window, wins[4].Label())
	}
	if len(rep.finalizeCalls) != 1 {
		t.Fatalf("finalize calls=%d want 1", len(rep.finalizeCalls))
	}
	if !errors.Is(rep.finalizeCalls[0].Err, errSentinel) {
		t.Fatalf("finalize err=%v", rep.finalizeCalls[0].Err)
	}
}

func TestRun_CtxCancel_ReturnsResumeToken(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)
	fk, rep, w := newFake(now)

	opts := stdOpts(now)
	opts.StartDate = mid(2026, 4, 2)
	opts.EndDate = mid(2026, 5, 3)
	opts.Writer = w
	opts.Reporter = rep
	opts.Clock = fk

	ctx, cancel := context.WithCancel(context.Background())
	wins := SplitWindows(opts.StartDate, opts.EndDate, opts.MaxWindowDays)

	// Fetcher: 1번째/2번째 윈도우는 정상, 3번째 진입 시 cancel하고 ctx.Err 반환.
	callCount := 0
	opts.Fetcher = func(c context.Context, win Window, _ PageState) (Page, error) {
		if cerr := c.Err(); cerr != nil {
			return Page{}, cerr
		}
		callCount++
		if win.Label() == wins[2].Label() {
			cancel()
			return Page{}, c.Err()
		}
		return Page{Records: []json.RawMessage{raw(`{"x":1}`)}, Next: nil}, nil
	}

	res, err := Run(ctx, opts)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v want context.Canceled", err)
	}
	if res.RecordsWritten != 2 {
		t.Fatalf("records=%d want 2", res.RecordsWritten)
	}
	if res.ResumeToken == "" {
		t.Fatal("want resume token on cancel")
	}
	tok, _ := DecodeToken(res.ResumeToken)
	if tok.Window != wins[2].Label() {
		t.Fatalf("token window=%q want %q", tok.Window, wins[2].Label())
	}
	if len(rep.finalizeCalls) != 1 || !errors.Is(rep.finalizeCalls[0].Err, context.Canceled) {
		t.Fatalf("finalize=%+v", rep.finalizeCalls)
	}
}

func TestRun_MaxPages(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)
	fk, rep, w := newFake(now)
	fetcher := newFakeFetcher()

	opts := stdOpts(now)
	opts.MaxPages = 2
	opts.Fetcher = fetcher.Fetch
	opts.Writer = w
	opts.Reporter = rep
	opts.Clock = fk

	wins := SplitWindows(opts.StartDate, opts.EndDate, opts.MaxWindowDays)
	label := wins[0].Label()
	// 무한 페이지 시뮬레이션: 3개 페이지 정의해두고 MaxPages=2.
	fetcher.set(label, nil, Page{Records: []json.RawMessage{raw(`{"p":1}`)}, Next: raw(`"s1"`)})
	fetcher.set(label, raw(`"s1"`), Page{Records: []json.RawMessage{raw(`{"p":2}`)}, Next: raw(`"s2"`)})
	fetcher.set(label, raw(`"s2"`), Page{Records: []json.RawMessage{raw(`{"p":3}`)}, Next: raw(`"s3"`)})

	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if res.RecordsWritten != 2 {
		t.Fatalf("records=%d want 2", res.RecordsWritten)
	}
	if res.ResumeToken == "" {
		t.Fatal("want resume token on max-pages exit")
	}
	tok, derr := DecodeToken(res.ResumeToken)
	if derr != nil {
		t.Fatalf("decode: %v", derr)
	}
	if tok.Window != label {
		t.Fatalf("token window=%q want %q", tok.Window, label)
	}
	if string(tok.State) != `"s2"` {
		t.Fatalf("token state=%q want %q", string(tok.State), `"s2"`)
	}
}

func TestRun_ResumeFromWindow(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)
	fk, rep, w := newFake(now)
	fetcher := newFakeFetcher()

	opts := stdOpts(now)
	opts.StartDate = mid(2026, 4, 2)
	opts.EndDate = mid(2026, 5, 3)
	opts.Fetcher = fetcher.Fetch
	opts.Writer = w
	opts.Reporter = rep
	opts.Clock = fk

	wins := SplitWindows(opts.StartDate, opts.EndDate, opts.MaxWindowDays)
	if len(wins) != 31 {
		t.Fatalf("setup: 31 wins, got %d", len(wins))
	}
	opts.StartWindowDate = wins[9].Label() // 10번째 윈도우 (index 9).

	// 10번째부터 끝까지만 fetcher 응답 등록.
	for i := 9; i < len(wins); i++ {
		fetcher.set(wins[i].Label(), nil, Page{Records: []json.RawMessage{raw(`{"k":1}`)}, Next: nil})
	}

	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if res.RecordsWritten != 22 {
		t.Fatalf("records=%d want 22 (31-9)", res.RecordsWritten)
	}
	if len(rep.starts) == 0 {
		t.Fatal("no WindowStart calls")
	}
	first := rep.starts[0]
	if first.WindowIndex != 10 || first.Total != 31 {
		t.Fatalf("first WindowStart=%+v want index=10 total=31", first)
	}
}

func TestRun_ResumeFromWindow_NotFound(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)
	fk, rep, w := newFake(now)
	opts := stdOpts(now)
	opts.Fetcher = newFakeFetcher().Fetch
	opts.Writer = w
	opts.Reporter = rep
	opts.Clock = fk
	opts.StartWindowDate = "9999-12-31"

	_, err := Run(context.Background(), opts)
	if err == nil {
		t.Fatal("want err for unknown resume window")
	}
	if len(rep.starts) != 0 {
		t.Fatalf("no windows should start, got %d", len(rep.starts))
	}
	// Finalize는 호출되지 않는 경로 (validation 단계 종료). 검증 단계에서 즉시 반환.
	if len(rep.finalizeCalls) != 0 {
		t.Fatalf("validation error path should not call Finalize, got %d", len(rep.finalizeCalls))
	}
}

func TestRun_ResumeFromInitialState(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)
	fk, rep, w := newFake(now)

	opts := stdOpts(now)
	opts.StartDate = mid(2026, 4, 2)
	opts.EndDate = mid(2026, 5, 3)
	opts.Writer = w
	opts.Reporter = rep
	opts.Clock = fk

	wins := SplitWindows(opts.StartDate, opts.EndDate, opts.MaxWindowDays)
	resumeIdx := 4
	opts.StartWindowDate = wins[resumeIdx].Label()
	opts.InitialState = raw(`{"off":50}`)

	var firstSeenState PageState
	firstCall := true
	opts.Fetcher = func(_ context.Context, win Window, state PageState) (Page, error) {
		if firstCall {
			firstSeenState = state
			firstCall = false
		}
		// 첫 호출은 InitialState로 들어와야 함. 빈 윈도우로 종료.
		_ = win
		return Page{Records: nil, Next: nil}, nil
	}

	if _, err := Run(context.Background(), opts); err != nil {
		t.Fatalf("err=%v", err)
	}
	if string(firstSeenState) != `{"off":50}` {
		t.Fatalf("first state=%q want %q", string(firstSeenState), `{"off":50}`)
	}
}

func TestRun_ResumeFromInitialState_OnlyFirstWindow(t *testing.T) {
	t.Parallel()
	// InitialState는 첫 윈도우만 적용. 두 번째 윈도우는 nil로 시작.
	now := mid(2026, 5, 11)
	fk, rep, w := newFake(now)
	opts := stdOpts(now)
	opts.StartDate = mid(2026, 4, 2)
	opts.EndDate = mid(2026, 5, 3)
	opts.Writer = w
	opts.Reporter = rep
	opts.Clock = fk

	wins := SplitWindows(opts.StartDate, opts.EndDate, opts.MaxWindowDays)
	opts.StartWindowDate = wins[0].Label()
	opts.InitialState = raw(`{"off":50}`)

	statesByCall := []PageState{}
	opts.Fetcher = func(_ context.Context, _ Window, state PageState) (Page, error) {
		statesByCall = append(statesByCall, state)
		return Page{Records: nil, Next: nil}, nil
	}

	if _, err := Run(context.Background(), opts); err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(statesByCall) < 2 {
		t.Fatalf("not enough calls=%d", len(statesByCall))
	}
	if string(statesByCall[0]) != `{"off":50}` {
		t.Fatalf("first call state=%q", string(statesByCall[0]))
	}
	if statesByCall[1] != nil {
		t.Fatalf("second window first call state=%q want nil", string(statesByCall[1]))
	}
}

// --- validation errors ---------------------------------------------------

func TestRun_ValidationErrors(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)
	cases := []struct {
		name    string
		mutate  func(*Options)
		wantErr error
	}{
		{
			name: "start exceeds lookback",
			mutate: func(o *Options) {
				o.StartDate = now.AddDate(0, 0, -200)
				o.EndDate = mid(2026, 5, 10)
			},
			wantErr: ErrLookbackExceeded,
		},
		{
			name: "start >= end",
			mutate: func(o *Options) {
				o.StartDate = mid(2026, 5, 1)
				o.EndDate = mid(2026, 5, 1)
			},
			wantErr: ErrStartAfterEnd,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fk, rep, w := newFake(now)
			opts := stdOpts(now)
			opts.Fetcher = newFakeFetcher().Fetch
			opts.Writer = w
			opts.Reporter = rep
			opts.Clock = fk
			tc.mutate(&opts)
			res, err := Run(context.Background(), opts)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v want errors.Is %v", err, tc.wantErr)
			}
			if res.RecordsWritten != 0 {
				t.Fatalf("records=%d want 0", res.RecordsWritten)
			}
			if res.ResumeToken != "" {
				t.Fatalf("resume token=%q want empty", res.ResumeToken)
			}
			// Validation 실패는 Finalize를 호출하지 않음.
			if len(rep.finalizeCalls) != 0 {
				t.Fatalf("finalize calls=%d want 0 on validation error", len(rep.finalizeCalls))
			}
		})
	}
}

func TestRun_EndDateClamp(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)
	fk, rep, w := newFake(now)
	fetcher := newFakeFetcher()

	opts := stdOpts(now)
	opts.StartDate = mid(2026, 5, 9)
	opts.EndDate = now.Add(1 * time.Hour) // future → clamp to now
	opts.Fetcher = fetcher.Fetch
	opts.Writer = w
	opts.Reporter = rep
	opts.Clock = fk

	// 실제 사용될 EndDate는 now. SplitWindows([5-9, 5-11)) = 단일 윈도우 [5-9, 5-11).
	wins := SplitWindows(opts.StartDate, now, opts.MaxWindowDays)
	for _, win := range wins {
		fetcher.set(win.Label(), nil, Page{Records: []json.RawMessage{raw(`{"x":1}`)}, Next: nil})
	}

	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("err=%v (clamp shouldn't error)", err)
	}
	if res.RecordsWritten != 1 {
		t.Fatalf("records=%d want 1 (단일 윈도우)", res.RecordsWritten)
	}
}

func TestRun_EmptyResult(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)
	fk, rep, w := newFake(now)
	fetcher := newFakeFetcher()

	opts := stdOpts(now)
	opts.Fetcher = fetcher.Fetch
	opts.Writer = w
	opts.Reporter = rep
	opts.Clock = fk

	wins := SplitWindows(opts.StartDate, opts.EndDate, opts.MaxWindowDays)
	fetcher.set(wins[0].Label(), nil, Page{Records: nil, Next: nil})

	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if res.RecordsWritten != 0 {
		t.Fatalf("records=%d want 0", res.RecordsWritten)
	}
	if res.ResumeToken != "" {
		t.Fatalf("resume token=%q want empty", res.ResumeToken)
	}
	// WindowStart + (PageProgress 1회) + WindowDone + Finalize 호출.
	if len(rep.starts) != 1 {
		t.Fatalf("starts=%d want 1", len(rep.starts))
	}
	if len(rep.dones) != 1 {
		t.Fatalf("dones=%d want 1", len(rep.dones))
	}
	if len(rep.pages) != 1 {
		t.Fatalf("pages=%d want 1 (empty page still calls PageProgress)", len(rep.pages))
	}
	if rep.pages[0].Cumulative != 0 {
		t.Fatalf("cumulative=%d want 0", rep.pages[0].Cumulative)
	}
}

func TestRun_WriterError(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)
	fk, rep, _ := newFake(now)
	fetcher := newFakeFetcher()
	werr := errors.New("disk full")
	w := &recordingWriter{writeErr: werr} // 첫 WriteRecord에서 에러

	opts := stdOpts(now)
	opts.Fetcher = fetcher.Fetch
	opts.Writer = w
	opts.Reporter = rep
	opts.Clock = fk

	wins := SplitWindows(opts.StartDate, opts.EndDate, opts.MaxWindowDays)
	fetcher.set(wins[0].Label(), nil, Page{Records: []json.RawMessage{raw(`{"x":1}`)}, Next: nil})

	res, err := Run(context.Background(), opts)
	if !errors.Is(err, werr) {
		t.Fatalf("err=%v want %v", err, werr)
	}
	if res.RecordsWritten != 0 {
		t.Fatalf("records=%d want 0", res.RecordsWritten)
	}
	if res.ResumeToken == "" {
		t.Fatal("want resume token on writer error")
	}
	tok, _ := DecodeToken(res.ResumeToken)
	if tok.Window != wins[0].Label() {
		t.Fatalf("token window=%q want %q", tok.Window, wins[0].Label())
	}
}

func TestRun_FlushError(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)
	fk, rep, _ := newFake(now)
	fetcher := newFakeFetcher()
	ferr := errors.New("flush fail")
	w := &recordingWriter{flushErr: ferr}

	opts := stdOpts(now)
	opts.Fetcher = fetcher.Fetch
	opts.Writer = w
	opts.Reporter = rep
	opts.Clock = fk

	wins := SplitWindows(opts.StartDate, opts.EndDate, opts.MaxWindowDays)
	fetcher.set(wins[0].Label(), nil, Page{Records: []json.RawMessage{raw(`{"x":1}`)}, Next: nil})

	res, err := Run(context.Background(), opts)
	if !errors.Is(err, ferr) {
		t.Fatalf("err=%v want %v", err, ferr)
	}
	// 1 record는 메모리에 들어왔으나 flush 실패 → records count는 1, ResumeToken은 발급.
	if res.RecordsWritten != 1 {
		t.Fatalf("records=%d want 1", res.RecordsWritten)
	}
	if res.ResumeToken == "" {
		t.Fatal("want resume token on flush error")
	}
}

func TestRun_FinalizeAlwaysCalled(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)

	type scenario struct {
		name      string
		setup     func(*Options, *fakeFetcher)
		wantErrIs error // nil이면 성공 경로
	}
	scenarios := []scenario{
		{
			name: "성공",
			setup: func(o *Options, ff *fakeFetcher) {
				wins := SplitWindows(o.StartDate, o.EndDate, o.MaxWindowDays)
				ff.set(wins[0].Label(), nil, Page{Records: []json.RawMessage{raw(`{}`)}, Next: nil})
			},
		},
		{
			name: "fetcher 에러",
			setup: func(o *Options, ff *fakeFetcher) {
				_ = o
				ff.err = errors.New("boom")
			},
			wantErrIs: errors.New("boom"), // sentinel; 비교는 별도
		},
		{
			name: "max-pages",
			setup: func(o *Options, ff *fakeFetcher) {
				o.MaxPages = 1
				wins := SplitWindows(o.StartDate, o.EndDate, o.MaxWindowDays)
				ff.set(wins[0].Label(), nil, Page{Records: []json.RawMessage{raw(`{}`)}, Next: raw(`"a"`)})
			},
		},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			t.Parallel()
			fk, rep, w := newFake(now)
			ff := newFakeFetcher()
			opts := stdOpts(now)
			opts.Fetcher = ff.Fetch
			opts.Writer = w
			opts.Reporter = rep
			opts.Clock = fk
			sc.setup(&opts, ff)
			_, _ = Run(context.Background(), opts)
			if got := len(rep.finalizeCalls); got != 1 {
				t.Fatalf("finalize calls=%d want 1", got)
			}
		})
	}
}

// --- ValidatePageSize / ValidateThrottle ---------------------------------

func TestValidatePageSize(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		value   int
		hardMax int
		wantErr bool
	}{
		{"OK 작은 값", 10, 100, false},
		{"OK 경계값 (max)", 100, 100, false},
		{"NG 초과", 101, 100, true},
		{"OK 0 (use default)", 0, 100, false},
		{"OK 음수", -1, 100, false},
		{"NG hardMax 0", 50, 0, true},
		{"NG hardMax 음수", 50, -1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePageSize("pageSize", tc.value, tc.hardMax)
			if tc.wantErr && err == nil {
				t.Fatalf("want err, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
		})
	}
}

func TestValidateThrottle(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		d       time.Duration
		minD    time.Duration
		wantErr bool
	}{
		{"OK 0 (use default)", 0, 100 * time.Millisecond, false},
		{"OK 음수 (use default)", -1 * time.Millisecond, 100 * time.Millisecond, false},
		{"OK 경계값", 100 * time.Millisecond, 100 * time.Millisecond, false},
		{"OK 큰 값", 500 * time.Millisecond, 100 * time.Millisecond, false},
		{"NG 너무 작음", 50 * time.Millisecond, 100 * time.Millisecond, true},
		{"OK minD<=0 → 100ms default 적용", 200 * time.Millisecond, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateThrottle(tc.d, tc.minD)
			if tc.wantErr && err == nil {
				t.Fatalf("want err")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
		})
	}
}

// --- 기타 ---------------------------------------------------------------

func TestRun_NilFetcher(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)
	_, rep, w := newFake(now)
	opts := stdOpts(now)
	opts.Writer = w
	opts.Reporter = rep
	// Fetcher 누락.
	_, err := Run(context.Background(), opts)
	if err == nil {
		t.Fatal("want err")
	}
}

func TestRun_NilWriter(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)
	_, rep, _ := newFake(now)
	opts := stdOpts(now)
	opts.Reporter = rep
	opts.Fetcher = newFakeFetcher().Fetch
	_, err := Run(context.Background(), opts)
	if err == nil {
		t.Fatal("want err")
	}
}

func TestRun_DefaultThrottleApplied(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)
	fk, rep, w := newFake(now)
	fetcher := newFakeFetcher()

	opts := stdOpts(now)
	opts.Throttle = 0 // → DefaultThrottle
	opts.Fetcher = fetcher.Fetch
	opts.Writer = w
	opts.Reporter = rep
	opts.Clock = fk

	wins := SplitWindows(opts.StartDate, opts.EndDate, opts.MaxWindowDays)
	fetcher.set(wins[0].Label(), nil, Page{Records: []json.RawMessage{raw(`{"a":1}`)}, Next: raw(`"x"`)})
	fetcher.set(wins[0].Label(), raw(`"x"`), Page{Records: []json.RawMessage{raw(`{"a":2}`)}, Next: nil})

	if _, err := Run(context.Background(), opts); err != nil {
		t.Fatalf("err=%v", err)
	}
	sleeps := fk.Sleeps()
	if len(sleeps) != 1 || sleeps[0] != DefaultThrottle {
		t.Fatalf("sleeps=%v want [%v]", sleeps, DefaultThrottle)
	}
}

func TestRun_DefaultClockApplied(t *testing.T) {
	t.Parallel()
	// Clock nil → realClock 적용. 페이지 0개 윈도우로 sleep을 피하면 빠르게 끝남.
	now := time.Now().UTC()
	rep := &recordingReporter{}
	w := &recordingWriter{}

	opts := stdOpts(now)
	opts.Fetcher = func(_ context.Context, _ Window, _ PageState) (Page, error) {
		return Page{Records: nil, Next: nil}, nil
	}
	opts.Writer = w
	opts.Reporter = rep
	// Clock = nil → Real() default.

	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if res.RecordsWritten != 0 {
		t.Fatalf("records=%d want 0", res.RecordsWritten)
	}
}

func TestRun_CtxCancel_BeforeFirstWindow(t *testing.T) {
	t.Parallel()
	now := mid(2026, 5, 11)
	fk, rep, w := newFake(now)
	opts := stdOpts(now)
	opts.Fetcher = newFakeFetcher().Fetch
	opts.Writer = w
	opts.Reporter = rep
	opts.Clock = fk

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 즉시 취소

	res, err := Run(ctx, opts)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v want context.Canceled", err)
	}
	if res.RecordsWritten != 0 {
		t.Fatalf("records=%d want 0", res.RecordsWritten)
	}
	// 첫 윈도우 진입 전 cancel → ResumeToken은 첫 윈도우 라벨.
	if res.ResumeToken == "" {
		t.Fatal("want resume token")
	}
	if len(rep.finalizeCalls) != 1 || !errors.Is(rep.finalizeCalls[0].Err, context.Canceled) {
		t.Fatalf("finalize=%+v", rep.finalizeCalls)
	}
}

// ensure progress.Reporter is satisfied by recordingReporter.
var _ progress.Reporter = (*recordingReporter)(nil)

// ensure RowWriter is satisfied by recordingWriter.
var _ RowWriter = (*recordingWriter)(nil)
