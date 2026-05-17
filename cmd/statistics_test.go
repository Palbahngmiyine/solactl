package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/solapi/solactl/pkg/client"
	"github.com/solapi/solactl/pkg/exporter"
)

// setupStatisticsExportTest wires up an httptest server, capture buffers, tmp dir.
func setupStatisticsExportTest(t *testing.T, handler http.HandlerFunc) (stderr *bytes.Buffer, tmpDir string) {
	t.Helper()
	resetFlags()
	resetStatisticsExportFlags()
	resetPflagChanged(statisticsExportDailyCmd)

	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	c := &client.Client{
		HTTPClient:      ts.Client(),
		APIKey:          "testkey",
		APISecret:       "testsecret",
		MaxRetries:      0,
		BaseDelay:       time.Millisecond,
		BaseURLOverride: ts.URL,
	}
	clientOverride = c

	var outBuf, errBuf bytes.Buffer
	outWriter = &outBuf
	errWriter = &errBuf

	tmpDir = t.TempDir()

	t.Cleanup(func() {
		clientOverride = nil
		outWriter = nil
		errWriter = nil
		resetStatisticsExportFlags()
		resetPflagChanged(statisticsExportDailyCmd)
	})
	return &errBuf, tmpDir
}

// runStatsExport executes the statistics export-daily command.
func runStatsExport(args ...string) error {
	full := append([]string{"statistics", "export-daily"}, args...)
	rootCmd.SetArgs(full)
	return rootCmd.Execute()
}

// recentDateUTC: ISO date N days before today (UTC) — 활용 가능 범위.
func recentDateUTC(daysAgo int) string {
	return time.Now().UTC().AddDate(0, 0, -daysAgo).Format("2006-01-02")
}

func TestStatisticsExportDaily_CSV_SingleWindow(t *testing.T) {
	_, tmpDir := setupStatisticsExportTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method=%s want GET", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/messages/v4/statistics/daily") {
			t.Errorf("path=%s", r.URL.Path)
		}
		resp := `[{
			"accountId":"AC1","date":"2026-05-09T00:00:00.000Z",
			"count":{"SMS":10,"LMS":5},
			"prepaid":true,"balance":100.5,"point":50,"profit":25,
			"refund":{"balance":0,"point":0}
		}]`
		_, _ = io.WriteString(w, resp)
	})

	outPath := filepath.Join(tmpDir, "out.csv")
	err := runStatsExport(
		"--output", outPath,
		"--start-date", recentDateUTC(1),
		"--end-date", recentDateUTC(0),
		"--throttle", "100ms",
		"--progress", "off",
	)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	rows := mustReadCSV(t, outPath)
	if len(rows) != 2 { // header + 1 record
		t.Fatalf("rows=%d want 2", len(rows))
	}
	wantHeaders := append(slices.Clone(statisticsCSVFixedHeaders), "count_LMS", "count_SMS")
	if len(rows[0]) != len(wantHeaders) {
		t.Fatalf("header len=%d want %d (%v)", len(rows[0]), len(wantHeaders), rows[0])
	}
	for i, h := range wantHeaders {
		if rows[0][i] != h {
			t.Errorf("header[%d]=%q want %q", i, rows[0][i], h)
		}
	}
	// Row 검증: count_LMS=5, count_SMS=10 등.
	rec := rows[1]
	headerToIdx := make(map[string]int, len(rows[0]))
	for i, h := range rows[0] {
		headerToIdx[h] = i
	}
	if rec[headerToIdx["accountId"]] != "AC1" {
		t.Errorf("accountId=%q", rec[headerToIdx["accountId"]])
	}
	if rec[headerToIdx["prepaid"]] != "true" {
		t.Errorf("prepaid=%q", rec[headerToIdx["prepaid"]])
	}
	if rec[headerToIdx["count_SMS"]] != "10" {
		t.Errorf("count_SMS=%q want 10", rec[headerToIdx["count_SMS"]])
	}
	if rec[headerToIdx["count_LMS"]] != "5" {
		t.Errorf("count_LMS=%q want 5", rec[headerToIdx["count_LMS"]])
	}
}

func TestStatisticsExportDaily_UnionHeader(t *testing.T) {
	// 다중 윈도우: 9일 범위 → 9개 윈도우. 호출별로 다른 count 키 셋을 반환하여 union을 검증.
	var calls atomic.Int64
	_, tmpDir := setupStatisticsExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		var body string
		switch n {
		case 1:
			body = `[{"accountId":"A","date":"2026-04-24T00:00:00.000Z","count":{"SMS":10},"balance":0,"point":0,"profit":0}]`
		case 5:
			body = `[{"accountId":"A","date":"2026-04-28T00:00:00.000Z","count":{"SMS":1,"RCS_SMS":2},"balance":0,"point":0,"profit":0}]`
		default:
			body = `[]`
		}
		_, _ = io.WriteString(w, body)
	})

	outPath := filepath.Join(tmpDir, "union.csv")
	err := runStatsExport(
		"--output", outPath,
		"--start-date", "2026-04-24",
		"--end-date", "2026-05-03",
		"--throttle", "100ms",
		"--progress", "off",
	)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	rows := mustReadCSV(t, outPath)
	if len(rows) < 3 { // header + 2 records
		t.Fatalf("rows=%d want >=3 (%v)", len(rows), rows)
	}
	// 헤더 marshal: count_RCS_SMS, count_SMS 둘 다 존재.
	header := rows[0]
	headerSet := make(map[string]int)
	for i, h := range header {
		headerSet[h] = i
	}
	if _, ok := headerSet["count_RCS_SMS"]; !ok {
		t.Fatalf("union header missing count_RCS_SMS: %v", header)
	}
	if _, ok := headerSet["count_SMS"]; !ok {
		t.Fatalf("union header missing count_SMS: %v", header)
	}
	// 첫 윈도우 record (count_SMS:10만 있는)의 count_RCS_SMS는 빈 셀.
	var firstRecord []string
	for i := 1; i < len(rows); i++ {
		if rows[i][headerSet["count_SMS"]] == "10" {
			firstRecord = rows[i]
			break
		}
	}
	if firstRecord == nil {
		t.Fatalf("could not find record with count_SMS=10: %v", rows)
	}
	if firstRecord[headerSet["count_RCS_SMS"]] != "" {
		t.Errorf("first window count_RCS_SMS=%q want empty", firstRecord[headerSet["count_RCS_SMS"]])
	}
}

func TestStatisticsExportDaily_OperationalLogScenario(t *testing.T) {
	// 31일 윈도우 split + 각 호출의 path/startDate/endDate 검증.
	var (
		mu       sync.Mutex
		startQs  []string
		endQs    []string
		callPath []string
	)
	_, tmpDir := setupStatisticsExportTest(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callPath = append(callPath, r.URL.Path)
		startQs = append(startQs, r.URL.Query().Get("startDate"))
		endQs = append(endQs, r.URL.Query().Get("endDate"))
		mu.Unlock()
		_, _ = io.WriteString(w, `[]`)
	})

	outPath := filepath.Join(tmpDir, "multi.jsonl")
	err := runStatsExport(
		"--output", outPath,
		"--format", "jsonl",
		"--start-date", "2026-04-02",
		"--end-date", "2026-05-03",
		"--throttle", "100ms",
		"--progress", "off",
	)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(callPath) != 31 {
		t.Fatalf("calls=%d want 31", len(callPath))
	}
	for _, p := range callPath {
		if strings.Contains(p, "messages-internal") {
			t.Fatalf("internal endpoint used: %s", p)
		}
		if !strings.HasSuffix(p, "/messages/v4/statistics/daily") {
			t.Errorf("unexpected path: %s", p)
		}
	}

	for i, s := range startQs {
		sTime, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatalf("start[%d] parse: %v", i, err)
		}
		if !sTime.Equal(sTime.Truncate(24 * time.Hour).UTC()) {
			t.Errorf("start[%d]=%s not at UTC midnight", i, s)
		}
		eTime, err := time.Parse(time.RFC3339, endQs[i])
		if err != nil {
			t.Fatalf("end[%d] parse: %v", i, err)
		}
		if d := eTime.Sub(sTime); d != 24*time.Hour {
			t.Errorf("window[%d] span=%v want 24h", i, d)
		}
		if i > 0 && endQs[i-1] != s {
			t.Errorf("window[%d] start=%s does not chain from prev end=%s", i, s, endQs[i-1])
		}
	}
}

func TestStatisticsExportDaily_LookbackExceeded(t *testing.T) {
	called := atomic.Int64{}
	_, tmpDir := setupStatisticsExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		called.Add(1)
		_, _ = io.WriteString(w, `[]`)
	})
	outPath := filepath.Join(tmpDir, "no.csv")
	oldDate := time.Now().UTC().AddDate(0, 0, -200).Format("2006-01-02")
	err := runStatsExport(
		"--output", outPath,
		"--start-date", oldDate,
		"--end-date", recentDateUTC(0),
		"--throttle", "100ms", "--progress", "off",
	)
	if err == nil {
		t.Fatal("expected lookback error")
	}
	if !errors.Is(err, exporter.ErrLookbackExceeded) {
		t.Errorf("err=%v want ErrLookbackExceeded", err)
	}
	if called.Load() != 0 {
		t.Errorf("API called %d times before validation", called.Load())
	}
	if _, err := os.Stat(outPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("file should not be created on validation error: %v", err)
	}
}

func TestStatisticsExportDaily_RequiredDates(t *testing.T) {
	// --start-date 누락.
	t.Run("missing_start", func(t *testing.T) {
		_, tmpDir := setupStatisticsExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `[]`)
		})
		outPath := filepath.Join(tmpDir, "ns.csv")
		err := runStatsExport(
			"--output", outPath,
			"--end-date", recentDateUTC(0),
			"--throttle", "100ms", "--progress", "off",
		)
		if err == nil {
			t.Fatal("expected --start-date required error")
		}
		if !strings.Contains(err.Error(), "start-date") {
			t.Errorf("error should mention start-date: %v", err)
		}
	})
	// --end-date 누락.
	t.Run("missing_end", func(t *testing.T) {
		_, tmpDir := setupStatisticsExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `[]`)
		})
		outPath := filepath.Join(tmpDir, "ne.csv")
		err := runStatsExport(
			"--output", outPath,
			"--start-date", recentDateUTC(2),
			"--throttle", "100ms", "--progress", "off",
		)
		if err == nil {
			t.Fatal("expected --end-date required error")
		}
		if !strings.Contains(err.Error(), "end-date") {
			t.Errorf("error should mention end-date: %v", err)
		}
	})
}

func TestStatisticsExportDaily_PageSizeUpperBound(t *testing.T) {
	_, tmpDir := setupStatisticsExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[]`)
	})
	outPath := filepath.Join(tmpDir, "nope.csv")
	err := runStatsExport(
		"--output", outPath,
		"--start-date", recentDateUTC(1), "--end-date", recentDateUTC(0),
		"--page-size", "500",
		"--throttle", "100ms", "--progress", "off",
	)
	if err == nil {
		t.Fatal("expected page-size error")
	}
	if !strings.Contains(err.Error(), "exceeds max") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStatisticsExportDaily_ThrottleMinimum(t *testing.T) {
	_, tmpDir := setupStatisticsExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[]`)
	})
	outPath := filepath.Join(tmpDir, "nope.csv")
	err := runStatsExport(
		"--output", outPath,
		"--start-date", recentDateUTC(1), "--end-date", recentDateUTC(0),
		"--throttle", "50ms", "--progress", "off",
	)
	if err == nil {
		t.Fatal("expected throttle error")
	}
	if !strings.Contains(err.Error(), "throttle") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStatisticsExportDaily_JSONL(t *testing.T) {
	// 8일 범위 → 8개 윈도우 → 호출 8회 → 적어도 2 라인 이상.
	var calls atomic.Int64
	_, tmpDir := setupStatisticsExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		i := calls.Add(1)
		body := fmt.Sprintf(`[{"accountId":"A%d","date":"2026-05-09T00:00:00.000Z","count":{"SMS":%d},"balance":0,"point":0,"profit":0}]`, i, i)
		_, _ = io.WriteString(w, body)
	})

	outPath := filepath.Join(tmpDir, "out.jsonl")
	startDate := time.Now().UTC().AddDate(0, 0, -8).Format("2006-01-02")
	endDate := time.Now().UTC().Format("2006-01-02")
	err := runStatsExport(
		"--output", outPath,
		"--format", "jsonl",
		"--start-date", startDate,
		"--end-date", endDate,
		"--throttle", "100ms",
		"--progress", "off",
	)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected multiple jsonl lines, got %d", len(lines))
	}
	for i, l := range lines {
		var probe map[string]any
		if err := json.Unmarshal([]byte(l), &probe); err != nil {
			t.Fatalf("line %d not valid JSON: %v", i, err)
		}
	}
}

func TestStatisticsExportDaily_JSONAppendRejected(t *testing.T) {
	_, tmpDir := setupStatisticsExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[]`)
	})
	outPath := filepath.Join(tmpDir, "out.json")
	if err := os.WriteFile(outPath, []byte("[]"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	err := runStatsExport(
		"--output", outPath,
		"--start-date", recentDateUTC(1), "--end-date", recentDateUTC(0),
		"--format", "json", "--append",
		"--throttle", "100ms", "--progress", "off",
	)
	if err == nil {
		t.Fatal("expected error: json + append not allowed")
	}
	if !strings.Contains(err.Error(), "--append") {
		t.Errorf("error should mention --append: %v", err)
	}
}

func TestStatisticsExportDaily_OffsetPagination(t *testing.T) {
	// 단일 윈도우, page-size 2. offset 0: 2개 (full), offset 2: 1개 (partial → 종료).
	var calls atomic.Int64
	var capturedOffsets []string
	var mu sync.Mutex
	_, tmpDir := setupStatisticsExportTest(t, func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		mu.Lock()
		capturedOffsets = append(capturedOffsets, r.URL.Query().Get("offset"))
		mu.Unlock()
		switch n {
		case 1:
			_, _ = io.WriteString(w, `[
				{"accountId":"A","date":"2026-05-09T00:00:00.000Z","count":{"SMS":1},"balance":0,"point":0,"profit":0},
				{"accountId":"A","date":"2026-05-09T00:00:00.000Z","count":{"SMS":2},"balance":0,"point":0,"profit":0}
			]`)
		case 2:
			_, _ = io.WriteString(w, `[
				{"accountId":"A","date":"2026-05-09T00:00:00.000Z","count":{"SMS":3},"balance":0,"point":0,"profit":0}
			]`)
		default:
			t.Errorf("unexpected call %d", n)
			_, _ = io.WriteString(w, `[]`)
		}
	})

	outPath := filepath.Join(tmpDir, "offset.csv")
	err := runStatsExport(
		"--output", outPath,
		"--start-date", recentDateUTC(1), "--end-date", recentDateUTC(0),
		"--page-size", "2",
		"--throttle", "100ms", "--progress", "off",
	)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("calls=%d want 2", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(capturedOffsets) != 2 {
		t.Fatalf("captured offsets=%v", capturedOffsets)
	}
	// 첫 호출은 offset 없음 ("" because 0 is skipped).
	if capturedOffsets[0] != "" {
		t.Errorf("first call offset=%q want empty", capturedOffsets[0])
	}
	// 두번째 호출은 offset=2.
	if capturedOffsets[1] != "2" {
		t.Errorf("second call offset=%q want 2", capturedOffsets[1])
	}
	rows := mustReadCSV(t, outPath)
	// 헤더 + 3 records.
	if len(rows) != 4 {
		t.Fatalf("rows=%d want 4", len(rows))
	}
}

func TestStatisticsExportDaily_EmptyResultEndsWindow(t *testing.T) {
	// page-size 30, 첫 응답에 2개 (< page-size) → 다음 페이지 호출 없음.
	var calls atomic.Int64
	_, tmpDir := setupStatisticsExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, `[
			{"accountId":"A","date":"2026-05-09T00:00:00.000Z","count":{"SMS":1},"balance":0,"point":0,"profit":0},
			{"accountId":"A","date":"2026-05-09T00:00:00.000Z","count":{"SMS":2},"balance":0,"point":0,"profit":0}
		]`)
	})

	outPath := filepath.Join(tmpDir, "empty.csv")
	err := runStatsExport(
		"--output", outPath,
		"--start-date", recentDateUTC(1), "--end-date", recentDateUTC(0),
		"--throttle", "100ms", "--progress", "off",
	)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("calls=%d want 1 (partial page should end window)", got)
	}
}

func TestStatisticsExportDaily_PrepaidFilter(t *testing.T) {
	t.Run("valid_true", func(t *testing.T) {
		var captured atomic.Value
		_, tmpDir := setupStatisticsExportTest(t, func(w http.ResponseWriter, r *http.Request) {
			if captured.Load() == nil {
				captured.Store(r.URL.Query().Get("prepaid"))
			}
			_, _ = io.WriteString(w, `[]`)
		})
		outPath := filepath.Join(tmpDir, "p.csv")
		err := runStatsExport(
			"--output", outPath,
			"--start-date", recentDateUTC(1), "--end-date", recentDateUTC(0),
			"--prepaid", "true",
			"--throttle", "100ms", "--progress", "off",
		)
		if err != nil {
			t.Fatalf("execute: %v", err)
		}
		got, _ := captured.Load().(string)
		if got != "true" {
			t.Errorf("prepaid query=%q want true", got)
		}
	})
	t.Run("valid_false", func(t *testing.T) {
		var captured atomic.Value
		_, tmpDir := setupStatisticsExportTest(t, func(w http.ResponseWriter, r *http.Request) {
			if captured.Load() == nil {
				captured.Store(r.URL.Query().Get("prepaid"))
			}
			_, _ = io.WriteString(w, `[]`)
		})
		outPath := filepath.Join(tmpDir, "p2.csv")
		err := runStatsExport(
			"--output", outPath,
			"--start-date", recentDateUTC(1), "--end-date", recentDateUTC(0),
			"--prepaid", "false",
			"--throttle", "100ms", "--progress", "off",
		)
		if err != nil {
			t.Fatalf("execute: %v", err)
		}
		got, _ := captured.Load().(string)
		if got != "false" {
			t.Errorf("prepaid query=%q want false", got)
		}
	})
	t.Run("invalid", func(t *testing.T) {
		_, tmpDir := setupStatisticsExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `[]`)
		})
		outPath := filepath.Join(tmpDir, "p3.csv")
		err := runStatsExport(
			"--output", outPath,
			"--start-date", recentDateUTC(1), "--end-date", recentDateUTC(0),
			"--prepaid", "bogus",
			"--throttle", "100ms", "--progress", "off",
		)
		if err == nil {
			t.Fatal("expected --prepaid invalid value error")
		}
		if !strings.Contains(err.Error(), "prepaid") {
			t.Errorf("error should mention prepaid: %v", err)
		}
	})
	t.Run("empty_omitted_from_query", func(t *testing.T) {
		var hasPrepaid atomic.Bool
		_, tmpDir := setupStatisticsExportTest(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Has("prepaid") {
				hasPrepaid.Store(true)
			}
			_, _ = io.WriteString(w, `[]`)
		})
		outPath := filepath.Join(tmpDir, "p4.csv")
		err := runStatsExport(
			"--output", outPath,
			"--start-date", recentDateUTC(1), "--end-date", recentDateUTC(0),
			"--throttle", "100ms", "--progress", "off",
		)
		if err != nil {
			t.Fatalf("execute: %v", err)
		}
		if hasPrepaid.Load() {
			t.Errorf("prepaid query should be absent when flag is empty")
		}
	})
}

func TestStatisticsExportDaily_NoInternalEndpoint(t *testing.T) {
	var paths []string
	var mu sync.Mutex
	_, tmpDir := setupStatisticsExportTest(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		_, _ = io.WriteString(w, `[]`)
	})
	outPath := filepath.Join(tmpDir, "p.csv")
	err := runStatsExport(
		"--output", outPath,
		"--start-date", "2026-04-15",
		"--end-date", "2026-04-22",
		"--throttle", "100ms",
		"--progress", "off",
	)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(paths) == 0 {
		t.Fatal("no API calls made")
	}
	for _, p := range paths {
		if strings.Contains(p, "messages-internal") {
			t.Fatalf("internal endpoint used: %s", p)
		}
		if !strings.HasSuffix(p, "/messages/v4/statistics/daily") {
			t.Errorf("unexpected path: %s", p)
		}
	}
}

func TestStatisticsExportDaily_ResumeToken(t *testing.T) {
	// 첫 라운드: 두번째 호출에서 5xx로 중단 → resume-token 발급.
	// 재호출: 동일 윈도우/offset에서 진행.
	var calls atomic.Int64
	stderr, tmpDir := setupStatisticsExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		switch n {
		case 1:
			// page-size=2짜리 full page → next offset=2.
			_, _ = io.WriteString(w, `[
				{"accountId":"A","date":"2026-05-09T00:00:00.000Z","count":{"SMS":1},"balance":0,"point":0,"profit":0},
				{"accountId":"A","date":"2026-05-09T00:00:00.000Z","count":{"SMS":2},"balance":0,"point":0,"profit":0}
			]`)
		case 2:
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"errorCode":"X","errorMessage":"boom"}`))
		default:
			// 재개 단계: 종결.
			_, _ = io.WriteString(w, `[{"accountId":"A","date":"2026-05-09T00:00:00.000Z","count":{"SMS":3},"balance":0,"point":0,"profit":0}]`)
		}
	})

	outPath := filepath.Join(tmpDir, "resume.csv")
	startDate := recentDateUTC(1)
	endDate := recentDateUTC(0)
	err := runStatsExport(
		"--output", outPath,
		"--start-date", startDate, "--end-date", endDate,
		"--page-size", "2",
		"--throttle", "100ms", "--progress", "off",
	)
	if err == nil {
		t.Fatal("expected error from second-call 500")
	}
	se := stderr.String()
	if !strings.Contains(se, "--resume-token") {
		t.Fatalf("stderr should contain resume token hint:\n%s", se)
	}
	tok := extractResumeToken(se)
	if tok == "" {
		t.Fatalf("no token in stderr:\n%s", se)
	}

	// 두번째 라운드 - append + resume-token.
	resetStatisticsExportFlags()
	resetPflagChanged(statisticsExportDailyCmd)
	err = runStatsExport(
		"--output", outPath, "--append",
		"--start-date", startDate, "--end-date", endDate,
		"--page-size", "2",
		"--resume-token", tok,
		"--throttle", "100ms", "--progress", "off",
	)
	if err != nil {
		t.Fatalf("resume execute: %v", err)
	}
}

func TestStatisticsExportDaily_StdoutOutput(t *testing.T) {
	stderr, _ := setupStatisticsExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[{"accountId":"X","date":"2026-05-09T00:00:00.000Z","count":{"SMS":1},"balance":0,"point":0,"profit":0}]`)
	})
	err := runStatsExport(
		"--output", "-",
		"--format", "jsonl",
		"--start-date", recentDateUTC(1), "--end-date", recentDateUTC(0),
		"--throttle", "100ms", "--progress", "on",
	)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if outWriter == nil {
		t.Fatal("outWriter nil")
	}
	stdout := outWriter.(*bytes.Buffer).String()
	if !strings.Contains(stdout, `"accountId":"X"`) {
		t.Errorf("stdout missing JSONL output: %q", stdout)
	}
	// progress는 stderr로만.
	if strings.Contains(stdout, "다운로드 현황") {
		t.Errorf("progress leaked into stdout: %q", stdout)
	}
	if !strings.Contains(stderr.String(), "다운로드") {
		t.Errorf("progress not on stderr: %q", stderr.String())
	}
}

func TestStatisticsExportDaily_BOM(t *testing.T) {
	_, tmpDir := setupStatisticsExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[{"accountId":"X","date":"2026-05-09T00:00:00.000Z","count":{"SMS":1},"balance":0,"point":0,"profit":0}]`)
	})
	outPath := filepath.Join(tmpDir, "bom.csv")
	err := runStatsExport(
		"--output", outPath,
		"--start-date", recentDateUTC(1), "--end-date", recentDateUTC(0),
		"--bom",
		"--throttle", "100ms", "--progress", "off",
	)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(data) < 3 || data[0] != 0xEF || data[1] != 0xBB || data[2] != 0xBF {
		t.Errorf("missing UTF-8 BOM prefix: % X", data[:min(3, len(data))])
	}
}

func TestStatisticsExportDaily_5xxError_PartialDataPreserved(t *testing.T) {
	// 한 윈도우에서 첫 페이지 성공 + 두번째 페이지 5xx → 첫 페이지 데이터는 디스크에 보존.
	var calls atomic.Int64
	stderr, tmpDir := setupStatisticsExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			_, _ = io.WriteString(w, `[
				{"accountId":"A","date":"2026-05-09T00:00:00.000Z","count":{"SMS":1},"balance":0,"point":0,"profit":0},
				{"accountId":"A","date":"2026-05-09T00:00:00.000Z","count":{"SMS":2},"balance":0,"point":0,"profit":0}
			]`)
			return
		}
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"errorCode":"X","errorMessage":"boom"}`))
	})

	outPath := filepath.Join(tmpDir, "err.csv")
	err := runStatsExport(
		"--output", outPath,
		"--start-date", recentDateUTC(1), "--end-date", recentDateUTC(0),
		"--page-size", "2",
		"--throttle", "100ms", "--progress", "off",
	)
	if err == nil {
		t.Fatal("expected error from 500")
	}
	// 첫 페이지 데이터는 디스크에 (FinalizeWrite가 runErr 후에도 호출됨).
	rows := mustReadCSV(t, outPath)
	if len(rows) < 3 { // header + 2 records
		t.Fatalf("rows=%d want >=3 (header + first page records preserved)", len(rows))
	}
	if !strings.Contains(stderr.String(), "--resume-token") {
		t.Errorf("stderr missing resume-token hint: %q", stderr.String())
	}
}

// TestStatisticsCSVRowWriter_MemoryWarn은 union-header 누적 메모리 가드 회귀.
// Gemini 리뷰: 모든 record를 메모리에 보관하므로 OOM 위험이 있음.
// 임계치 도달 시 stderr에 1회만 경고를 출력해야 한다 (idempotency).
func TestStatisticsCSVRowWriter_MemoryWarn(t *testing.T) {
	t.Run("임계치 미만은 경고 없음", func(t *testing.T) {
		var stderr bytes.Buffer
		rw := &statisticsCSVRowWriter{
			w:                io.Discard,
			countKeys:        make(map[string]struct{}),
			memWarnWriter:    &stderr,
			memWarnThreshold: 5,
		}
		// 4건만 작성 (임계치 5 미만).
		for range 4 {
			rec := json.RawMessage(`{"date":"2026-05-09","accountId":"AC1","count":{"SMS":1}}`)
			if err := rw.WriteRecord(rec); err != nil {
				t.Fatalf("WriteRecord: %v", err)
			}
		}
		if stderr.Len() != 0 {
			t.Errorf("임계치 미만에서 경고 발생: %q", stderr.String())
		}
		if rw.memWarned {
			t.Error("memWarned가 true로 설정되면 안 됨")
		}
	})

	t.Run("임계치 도달 시 1회만 경고", func(t *testing.T) {
		var stderr bytes.Buffer
		rw := &statisticsCSVRowWriter{
			w:                io.Discard,
			countKeys:        make(map[string]struct{}),
			memWarnWriter:    &stderr,
			memWarnThreshold: 3,
		}
		// 10건 작성 — 임계치 3 초과 (idempotency 검증).
		rec := json.RawMessage(`{"date":"2026-05-09","accountId":"AC1","count":{"SMS":1}}`)
		for i := range 10 {
			if err := rw.WriteRecord(rec); err != nil {
				t.Fatalf("WriteRecord %d: %v", i, err)
			}
		}
		out := stderr.String()
		if !strings.Contains(out, "메모리에 누적된") {
			t.Errorf("경고 메시지 누락: %q", out)
		}
		if !strings.Contains(out, "3건") {
			t.Errorf("임계치 값 누락: %q", out)
		}
		// 1회만 출력 — "메모리에 누적된" 키워드는 정확히 1번만 등장해야 함.
		if cnt := strings.Count(out, "메모리에 누적된"); cnt != 1 {
			t.Errorf("경고 발생 횟수=%d, want 1 (1회만 출력해야 함)", cnt)
		}
	})

	t.Run("memWarnWriter nil이면 panic 없이 비활성", func(t *testing.T) {
		rw := &statisticsCSVRowWriter{
			w:                io.Discard,
			countKeys:        make(map[string]struct{}),
			memWarnWriter:    nil,
			memWarnThreshold: 1, // 즉시 임계치 초과 조건이지만 writer nil이라 무시.
		}
		// panic이 발생하면 테스트 실패.
		rec := json.RawMessage(`{"date":"2026-05-09","accountId":"AC1","count":{}}`)
		for range 5 {
			if err := rw.WriteRecord(rec); err != nil {
				t.Fatalf("WriteRecord: %v", err)
			}
		}
		if rw.memWarned {
			t.Error("writer nil인데 memWarned=true (가드 무력화)")
		}
	})

	t.Run("memWarnThreshold 0이면 비활성", func(t *testing.T) {
		var stderr bytes.Buffer
		rw := &statisticsCSVRowWriter{
			w:                io.Discard,
			countKeys:        make(map[string]struct{}),
			memWarnWriter:    &stderr,
			memWarnThreshold: 0,
		}
		rec := json.RawMessage(`{"date":"2026-05-09","accountId":"AC1","count":{}}`)
		for range 100 {
			if err := rw.WriteRecord(rec); err != nil {
				t.Fatalf("WriteRecord: %v", err)
			}
		}
		if stderr.Len() != 0 {
			t.Errorf("threshold=0인데 경고 발생: %q", stderr.String())
		}
	})

	t.Run("잘못된 JSON은 에러 반환하되 record 누적 없음", func(t *testing.T) {
		var stderr bytes.Buffer
		rw := &statisticsCSVRowWriter{
			w:                io.Discard,
			countKeys:        make(map[string]struct{}),
			memWarnWriter:    &stderr,
			memWarnThreshold: 1,
		}
		// 빈 객체 → 정상 디코드 → 1건 누적 → 임계치 1 도달 → 경고 1회
		if err := rw.WriteRecord(json.RawMessage(`{}`)); err != nil {
			t.Fatalf("first WriteRecord: %v", err)
		}
		if !strings.Contains(stderr.String(), "메모리에 누적된") {
			t.Errorf("첫 record 후 경고 없음: %q", stderr.String())
		}
		// 디코드 실패는 에러 반환 + 누적 없음.
		stderr.Reset()
		err := rw.WriteRecord(json.RawMessage(`{not valid json`))
		if err == nil {
			t.Error("잘못된 JSON에서 에러 기대")
		}
		// 이미 memWarned=true이므로 추가 경고 없음.
		if stderr.Len() != 0 {
			t.Errorf("디코드 실패 후 추가 출력: %q", stderr.String())
		}
	})
}

func resetStatisticsExportFlags() {
	statsExportFlagOutput = ""
	statsExportFlagFormat = "csv"
	statsExportFlagThrottle = statisticsExportThrottleDefault
	statsExportFlagPageSize = statisticsExportPageSizeDefault
	statsExportFlagMaxPages = 0
	statsExportFlagAppend = false
	statsExportFlagBOM = false
	statsExportFlagProgress = "auto"
	statsExportFlagNoProgress = false
	statsExportFlagResumeToken = ""
	statsExportFlagStartDate = ""
	statsExportFlagEndDate = ""
	statsExportFlagPrepaid = ""
	statsExportFlagOffset = 0
}
