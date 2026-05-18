package cmd

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/solapi/solactl/pkg/client"
	"github.com/solapi/solactl/pkg/exporter"
)

// setupMessagesExportTest wires up an httptest server, capture buffers, and a
// fresh tmp dir. flag 변수 reset만으로도 cobra Flags가 가리키는 포인터를 공유하지만,
// pflag의 Changed 비트는 별도로 리셋해야 MarkFlagRequired가 정확히 동작한다.
func setupMessagesExportTest(t *testing.T, handler http.HandlerFunc) (stderr *bytes.Buffer, tmpDir string) {
	t.Helper()
	resetFlags()
	resetMessagesExportFlags()
	resetPflagChanged(messagesExportCmd)

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
		resetMessagesExportFlags()
		resetPflagChanged(messagesExportCmd)
	})
	return &errBuf, tmpDir
}

// runExport executes the export command with the given args and returns the error.
func runExport(args ...string) error {
	full := append([]string{"messages", "export"}, args...)
	rootCmd.SetArgs(full)
	return rootCmd.Execute()
}

// recentDate returns an ISO date N days before today (UTC) — within lookback.
func recentDate(daysAgo int) string {
	return time.Now().UTC().AddDate(0, 0, -daysAgo).Format("2006-01-02")
}

func TestMessagesExport_CSV_SingleWindow_LegacyMap(t *testing.T) {
	_, tmpDir := setupMessagesExportTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method=%s want GET", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/messages/v4/list") {
			t.Errorf("path=%s", r.URL.Path)
		}
		resp := `{"messageList":{
			"MSG001":{"type":"SMS","statusCode":"2000","to":"01011112222","from":"01099998888","dateCreated":"2026-05-10T10:00:00"},
			"MSG002":{"type":"LMS","statusCode":"2000","to":"01033334444","from":"01099998888","dateCreated":"2026-05-10T11:00:00"}
		}}`
		_, _ = io.WriteString(w, resp)
	})

	outPath := filepath.Join(tmpDir, "out.csv")
	if err := runExport("--output", outPath, "--throttle", "100ms", "--progress", "off"); err != nil {
		t.Fatalf("execute: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	r := csv.NewReader(bytes.NewReader(data))
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("csv parse: %v", err)
	}
	if len(rows) != 3 { // header + 2 records
		t.Fatalf("rows=%d want 3", len(rows))
	}
	// 헤더 검증.
	for i, h := range messagesCSVHeaders {
		if rows[0][i] != h {
			t.Errorf("header[%d]=%q want %q", i, rows[0][i], h)
		}
	}
	// 정렬은 키 사전순: MSG001 → MSG002.
	if rows[1][0] != "MSG001" || rows[2][0] != "MSG002" {
		t.Errorf("messageId order=%q,%q want MSG001,MSG002", rows[1][0], rows[2][0])
	}
}

func TestMessagesExport_CSV_SingleWindow_ArrayResponse(t *testing.T) {
	_, tmpDir := setupMessagesExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		resp := `{"messages":[
			{"messageId":"M-A","type":"SMS","statusCode":"2000","to":"010"},
			{"messageId":"M-B","type":"LMS","statusCode":"2000","to":"011"}
		]}`
		_, _ = io.WriteString(w, resp)
	})

	outPath := filepath.Join(tmpDir, "out.csv")
	if err := runExport("--output", outPath, "--throttle", "100ms", "--progress", "off"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	rows := mustReadCSV(t, outPath)
	if len(rows) != 3 {
		t.Fatalf("rows=%d", len(rows))
	}
	if rows[1][0] != "M-A" || rows[2][0] != "M-B" {
		t.Errorf("ids=%q,%q", rows[1][0], rows[2][0])
	}
}

func TestMessagesExport_JSONL(t *testing.T) {
	// 8일 범위 → 8개 1일 윈도우.
	var calls atomic.Int64
	_, tmpDir := setupMessagesExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		i := calls.Add(1)
		resp := fmt.Sprintf(`{"messageList":{"M%d":{"type":"SMS","statusCode":"2000","to":"010"}}}`, i)
		_, _ = io.WriteString(w, resp)
	})

	outPath := filepath.Join(tmpDir, "out.jsonl")
	startDate := time.Now().UTC().AddDate(0, 0, -8).Format("2006-01-02")
	endDate := time.Now().UTC().Format("2006-01-02")
	err := runExport(
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

func TestMessagesExport_JSON(t *testing.T) {
	_, tmpDir := setupMessagesExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"messages":[{"messageId":"X","type":"SMS"},{"messageId":"Y","type":"LMS"}]}`)
	})
	outPath := filepath.Join(tmpDir, "out.json")
	if err := runExport("--output", outPath, "--format", "json", "--throttle", "100ms", "--progress", "off"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err != nil {
		t.Fatalf("invalid JSON array: %v (%s)", err, string(data))
	}
	if len(arr) != 2 {
		t.Errorf("array len=%d want 2", len(arr))
	}
}

func TestMessagesExport_JSON_AppendRejected(t *testing.T) {
	_, tmpDir := setupMessagesExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"messages":[]}`)
	})
	outPath := filepath.Join(tmpDir, "out.json")
	if err := os.WriteFile(outPath, []byte("[]"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	err := runExport("--output", outPath, "--format", "json", "--append", "--throttle", "100ms", "--progress", "off")
	if err == nil {
		t.Fatal("expected error: json + append not allowed")
	}
	if !strings.Contains(err.Error(), "--append") {
		t.Errorf("error should mention --append: %v", err)
	}
}

func TestMessagesExport_MultiWindowAutoSplit(t *testing.T) {
	// 31일 범위 → 31개 윈도우. 각 호출의 startDate/endDate가 1일 간격이며 UTC 자정 정렬.
	var (
		mu       sync.Mutex
		startQs  []string
		endQs    []string
		callPath []string
	)
	_, tmpDir := setupMessagesExportTest(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callPath = append(callPath, r.URL.Path)
		startQs = append(startQs, r.URL.Query().Get("startDate"))
		endQs = append(endQs, r.URL.Query().Get("endDate"))
		mu.Unlock()
		_, _ = io.WriteString(w, `{"messages":[]}`)
	})

	outPath := filepath.Join(tmpDir, "multi.jsonl")
	err := runExport(
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
		if !strings.HasSuffix(p, "/messages/v4/list") {
			t.Errorf("unexpected path: %s", p)
		}
	}

	prevEnd := ""
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
			t.Errorf("window[%d] start=%s does not chain from prev end=%s", i, s, prevEnd)
		}
		prevEnd = endQs[i]
	}
}

func TestMessagesExport_LookbackExceeded(t *testing.T) {
	called := atomic.Int64{}
	_, tmpDir := setupMessagesExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		called.Add(1)
		_, _ = io.WriteString(w, `{"messages":[]}`)
	})
	outPath := filepath.Join(tmpDir, "no.csv")
	oldDate := time.Now().UTC().AddDate(0, 0, -200).Format("2006-01-02")
	err := runExport("--output", outPath, "--start-date", oldDate, "--throttle", "100ms", "--progress", "off")
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
		// 파일이 생기지 말아야 — 단, openExportOutput는 운영적으로 검증 전에 호출되지 않으므로
		// 검증 실패 → file 미생성 보장.
		t.Errorf("file should not be created on validation error: %v", err)
	}
}

func TestMessagesExport_PageSizeUpperBound(t *testing.T) {
	_, tmpDir := setupMessagesExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"messages":[]}`)
	})
	outPath := filepath.Join(tmpDir, "nope.csv")
	err := runExport("--output", outPath, "--page-size", "500", "--throttle", "100ms", "--progress", "off")
	if err == nil {
		t.Fatal("expected page-size error")
	}
	if !strings.Contains(err.Error(), "exceeds max") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMessagesExport_ThrottleMinimum(t *testing.T) {
	_, tmpDir := setupMessagesExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"messages":[]}`)
	})
	outPath := filepath.Join(tmpDir, "nope.csv")
	err := runExport("--output", outPath, "--throttle", "50ms", "--progress", "off")
	if err == nil {
		t.Fatal("expected throttle error")
	}
	if !strings.Contains(err.Error(), "throttle") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMessagesExport_RequiredOutput(t *testing.T) {
	setupMessagesExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"messages":[]}`)
	})
	// --output 미지정 — cobra가 required flag 에러 반환.
	err := runExport("--throttle", "100ms", "--progress", "off")
	if err == nil {
		t.Fatal("expected required flag error")
	}
	if !strings.Contains(err.Error(), "output") {
		t.Errorf("error should mention output: %v", err)
	}
}

func TestMessagesExport_FileExistsWithoutAppend(t *testing.T) {
	_, tmpDir := setupMessagesExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"messages":[]}`)
	})
	outPath := filepath.Join(tmpDir, "exists.csv")
	if err := os.WriteFile(outPath, []byte("existing\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	err := runExport("--output", outPath, "--throttle", "100ms", "--progress", "off")
	if err == nil {
		t.Fatal("expected file-exists error")
	}
	if !strings.Contains(err.Error(), "이미 존재") {
		t.Errorf("error should mention file exists: %v", err)
	}
}

func TestMessagesExport_Append_HeaderMismatch(t *testing.T) {
	_, tmpDir := setupMessagesExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"messages":[]}`)
	})
	outPath := filepath.Join(tmpDir, "bad.csv")
	if err := os.WriteFile(outPath, []byte("wrong,header,here\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	err := runExport("--output", outPath, "--append", "--throttle", "100ms", "--progress", "off")
	if err == nil {
		t.Fatal("expected header mismatch error")
	}
}

func TestMessagesExport_Append_Success(t *testing.T) {
	_, tmpDir := setupMessagesExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"messages":[{"messageId":"NEW","type":"SMS"}]}`)
	})
	outPath := filepath.Join(tmpDir, "ok.csv")
	// 기존 파일: 정확한 헤더 + 1행.
	existing := strings.Join(messagesCSVHeaders, ",") + "\nOLD,SMS,,,,,,,,,,,,\n"
	if err := os.WriteFile(outPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	err := runExport("--output", outPath, "--append", "--throttle", "100ms", "--progress", "off")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	rows := mustReadCSV(t, outPath)
	if len(rows) != 3 { // 헤더 + OLD + NEW
		t.Fatalf("rows=%d want 3", len(rows))
	}
	if rows[1][0] != "OLD" || rows[2][0] != "NEW" {
		t.Errorf("ids=%q,%q", rows[1][0], rows[2][0])
	}
	// 헤더가 다시 쓰이지 않았는지 검증: 헤더 라인이 단 1회.
	data, _ := os.ReadFile(outPath)
	if c := strings.Count(string(data), "messageId,type,status,statusCode"); c != 1 {
		t.Errorf("header appears %d times, want 1", c)
	}
}

func TestMessagesExport_ResumeToken(t *testing.T) {
	// 첫 라운드: cancel을 통해 토큰을 받고, 두번째 라운드에서 재개.
	var calls atomic.Int64
	stderr, tmpDir := setupMessagesExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			_, _ = io.WriteString(w, `{"messages":[{"messageId":"P1","type":"SMS"}],"nextKey":"K2"}`)
			return
		}
		if n == 2 {
			// 두번째 호출에서 5xx — fetcher 에러로 토큰 발급.
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"errorCode":"X","errorMessage":"boom"}`))
			return
		}
		_, _ = io.WriteString(w, `{"messages":[{"messageId":"P3","type":"SMS"}]}`)
	})

	outPath := filepath.Join(tmpDir, "resume.csv")
	startDate := recentDate(1)
	endDate := time.Now().UTC().Format("2006-01-02")
	err := runExport("--output", outPath, "--start-date", startDate, "--end-date", endDate, "--throttle", "100ms", "--progress", "off")
	if err == nil {
		t.Fatal("expected error from second-call 500")
	}
	se := stderr.String()
	if !strings.Contains(se, "--resume-token") {
		t.Fatalf("stderr should contain resume token hint:\n%s", se)
	}
	// 두번째 라운드 — append + resume-token. 토큰 추출.
	tok := extractResumeToken(se)
	if tok == "" {
		t.Fatalf("no token in stderr:\n%s", se)
	}

	// 두번째 호출 사이클 — 동일 date range 유지해야 windows split 결과가 일치.
	resetMessagesExportFlags()
	resetPflagChanged(messagesExportCmd)
	err = runExport(
		"--output", outPath, "--append",
		"--start-date", startDate, "--end-date", endDate,
		"--resume-token", tok,
		"--throttle", "100ms", "--progress", "off",
	)
	if err != nil {
		t.Fatalf("resume execute: %v", err)
	}
	rows := mustReadCSV(t, outPath)
	// 헤더 + P1 (첫 라운드) + P3 (두번째 라운드).
	if len(rows) < 2 {
		t.Fatalf("rows=%d want >=2", len(rows))
	}
}

func TestMessagesExport_StartKey(t *testing.T) {
	var capturedKey atomic.Value
	_, tmpDir := setupMessagesExportTest(t, func(w http.ResponseWriter, r *http.Request) {
		// 첫 호출의 startKey만 캡처. 이후 호출은 nextKey 없는 응답으로 종료.
		if capturedKey.Load() == nil {
			capturedKey.Store(r.URL.Query().Get("startKey"))
		}
		_, _ = io.WriteString(w, `{"messages":[]}`)
	})
	outPath := filepath.Join(tmpDir, "k.csv")
	err := runExport("--output", outPath, "--start-key", "MSG999", "--throttle", "100ms", "--progress", "off")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	got, _ := capturedKey.Load().(string)
	if got != "MSG999" {
		t.Errorf("startKey=%q want MSG999", got)
	}
}

func TestMessagesExport_NoInternalEndpoint(t *testing.T) {
	var paths []string
	var mu sync.Mutex
	_, tmpDir := setupMessagesExportTest(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		_, _ = io.WriteString(w, `{"messages":[]}`)
	})
	outPath := filepath.Join(tmpDir, "p.csv")
	err := runExport(
		"--output", outPath,
		"--start-date", "2026-04-02",
		"--end-date", "2026-04-15",
		"--throttle", "100ms",
		"--progress", "off",
	)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, p := range paths {
		if strings.Contains(p, "messages-internal") {
			t.Fatalf("internal endpoint used: %s", p)
		}
	}
}

func TestMessagesExport_FiltersInQuery(t *testing.T) {
	var captured atomic.Value
	_, tmpDir := setupMessagesExportTest(t, func(w http.ResponseWriter, r *http.Request) {
		if captured.Load() == nil {
			captured.Store(r.URL.Query())
		}
		_, _ = io.WriteString(w, `{"messages":[]}`)
	})
	outPath := filepath.Join(tmpDir, "f.csv")
	err := runExport(
		"--output", outPath,
		"--type", "SMS",
		"--status-code", "4000",
		"--to", "01011112222",
		"--from", "01033334444",
		"--group-id", "GRP1",
		"--throttle", "100ms",
		"--progress", "off",
	)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	q, _ := captured.Load().(url.Values)
	if q.Get("type") != "SMS" {
		t.Errorf("type=%q", q.Get("type"))
	}
	if q.Get("statusCode") != "4000" {
		t.Errorf("statusCode=%q", q.Get("statusCode"))
	}
	if q.Get("to") != "01011112222" {
		t.Errorf("to=%q", q.Get("to"))
	}
	if q.Get("from") != "01033334444" {
		t.Errorf("from=%q", q.Get("from"))
	}
	if q.Get("groupId") != "GRP1" {
		t.Errorf("groupId=%q", q.Get("groupId"))
	}
}

func TestMessagesExport_StdoutOutput(t *testing.T) {
	stderr, _ := setupMessagesExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"messages":[{"messageId":"X1","type":"SMS"}]}`)
	})
	// progress=on을 의도적으로 켜서 stderr에만 쓰이는지 검증.
	err := runExport("--output", "-", "--format", "jsonl", "--throttle", "100ms", "--progress", "on")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if outWriter == nil {
		t.Fatal("outWriter nil")
	}
	stdout := outWriter.(*bytes.Buffer).String()
	if !strings.Contains(stdout, `"messageId":"X1"`) {
		t.Errorf("stdout missing JSONL output: %q", stdout)
	}
	// progress는 stderr로만 가야 함 — stdout에는 진행률 텍스트 없어야.
	if strings.Contains(stdout, "다운로드 현황") {
		t.Errorf("progress leaked into stdout: %q", stdout)
	}
	// stderr에는 적어도 finalize 한 줄이 들어가야.
	if !strings.Contains(stderr.String(), "다운로드") {
		t.Errorf("progress not on stderr: %q", stderr.String())
	}
}

func TestMessagesExport_BOM(t *testing.T) {
	_, tmpDir := setupMessagesExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"messages":[{"messageId":"K","type":"SMS"}]}`)
	})
	outPath := filepath.Join(tmpDir, "bom.csv")
	err := runExport("--output", outPath, "--bom", "--throttle", "100ms", "--progress", "off")
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

func TestMessagesExport_5xxError(t *testing.T) {
	var calls atomic.Int64
	stderr, tmpDir := setupMessagesExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			_, _ = io.WriteString(w, `{"messages":[{"messageId":"A","type":"SMS"}],"nextKey":"K2"}`)
			return
		}
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"errorCode":"X","errorMessage":"boom"}`))
	})

	outPath := filepath.Join(tmpDir, "err.csv")
	err := runExport("--output", outPath, "--throttle", "100ms", "--progress", "off")
	if err == nil {
		t.Fatal("expected error from 500")
	}
	// 첫 페이지 데이터는 디스크에 보존되어야 함.
	rows := mustReadCSV(t, outPath)
	if len(rows) < 2 {
		t.Fatalf("rows=%d want >=2 (header + first page)", len(rows))
	}
	if rows[1][0] != "A" {
		t.Errorf("first row id=%q want A", rows[1][0])
	}
	// stderr에 resume-token 안내.
	if !strings.Contains(stderr.String(), "--resume-token") {
		t.Errorf("stderr missing resume-token hint: %q", stderr.String())
	}
}

func TestMessagesExport_EnsureMessageID_Backfill(t *testing.T) {
	_, tmpDir := setupMessagesExportTest(t, func(w http.ResponseWriter, _ *http.Request) {
		// map 응답인데 record에 messageId 없음 → key로 채워져야.
		_, _ = io.WriteString(w, `{"messageList":{"BACKFILL-ID":{"type":"SMS","statusCode":"2000"}}}`)
	})
	outPath := filepath.Join(tmpDir, "bf.csv")
	if err := runExport("--output", outPath, "--throttle", "100ms", "--progress", "off"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	rows := mustReadCSV(t, outPath)
	if len(rows) != 2 {
		t.Fatalf("rows=%d", len(rows))
	}
	if rows[1][0] != "BACKFILL-ID" {
		t.Errorf("messageId=%q want BACKFILL-ID", rows[1][0])
	}
}

// --- helpers ---

func mustReadCSV(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = f.Close() }()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("csv read: %v", err)
	}
	return rows
}

// extractResumeToken parses the "--resume-token TOKEN" hint from stderr.
func extractResumeToken(stderr string) string {
	const marker = "--resume-token "
	_, rest, ok := strings.Cut(stderr, marker)
	if !ok {
		return ""
	}
	// 토큰은 공백/줄바꿈에서 끝남.
	end := strings.IndexAny(rest, " \n\t\r")
	if end < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:end])
}

func resetMessagesExportFlags() {
	msgExportFlagOutput = ""
	msgExportFlagFormat = "csv"
	msgExportFlagThrottle = messagesExportThrottleDefault
	msgExportFlagPageSize = messagesExportPageSizeDefault
	msgExportFlagMaxPages = 0
	msgExportFlagAppend = false
	msgExportFlagBOM = false
	msgExportFlagProgress = "auto"
	msgExportFlagNoProgress = false
	msgExportFlagResumeToken = ""
	msgExportFlagStartDate = ""
	msgExportFlagEndDate = ""
	msgExportFlagType = ""
	msgExportFlagStatus = ""
	msgExportFlagTo = ""
	msgExportFlagFrom = ""
	msgExportFlagGroupID = ""
	msgExportFlagStartKey = ""
}
