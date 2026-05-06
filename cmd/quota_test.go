package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/solapi/solactl/pkg/client"
)

func resetQuotaFlags() {
	resetFlags()
	quotaRequestFlagTarget = 0
	quotaRequestFlagReason = ""
	quotaListRequestsFlagStatus = ""
	quotaListRequestsFlagStartKey = ""
	quotaListRequestsFlagLimit = 20
}

// setupQuotaTest mirrors setupBalanceTest: stands up an httptest server,
// installs the test client, captures stdout and stderr.
func setupQuotaTest(t *testing.T, handler http.HandlerFunc) (stdout, stderr *bytes.Buffer) {
	t.Helper()
	resetQuotaFlags()

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

	t.Cleanup(func() {
		clientOverride = nil
		outWriter = nil
		errWriter = nil
		resetQuotaFlags()
	})

	return &outBuf, &errBuf
}

// ---------- quota get ----------

func TestQuotaGet_Success(t *testing.T) {
	stdout, _ := setupQuotaTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/quota/v1/me") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"accountId":"acc1","quota":5000,"min":50,"max":100000,"autoAdjustment":false,"overseasQuota":100,"dateCreated":"2026-05-01T00:00:00.000Z","dateUpdated":"2026-05-02T00:00:00.000Z"}`)
	})

	rootCmd.SetArgs([]string{"quota", "get"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := stdout.String()
	for _, want := range []string{"5,000", "100,000", "No", "100", "현재 한도", "자동 조정"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
}

func TestQuotaGet_JSON(t *testing.T) {
	stdout, _ := setupQuotaTest(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"accountId":"acc1","quota":5000,"min":50,"max":100000,"autoAdjustment":true,"overseasQuota":100}`)
	})

	rootCmd.SetArgs([]string{"quota", "get", "--json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput=%s", err, stdout.String())
	}
	if parsed["quota"] != float64(5000) {
		t.Errorf("quota: got %v", parsed["quota"])
	}
}

func TestQuotaGet_APIError(t *testing.T) {
	setupQuotaTest(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"errorCode":"Unauthorized","errorMessage":"인증 실패"}`))
	})

	rootCmd.SetArgs([]string{"quota", "get"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected error for API 401")
	}
}

func TestQuotaGet_MalformedJSON(t *testing.T) {
	setupQuotaTest(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{not json}`))
	})

	rootCmd.SetArgs([]string{"quota", "get"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "응답 파싱 실패") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

// ---------- quota request ----------

func TestQuotaRequest_Success(t *testing.T) {
	var capturedBody map[string]any
	stdout, _ := setupQuotaTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/quota/v1/me/system") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &capturedBody)

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"handleKey":"QT01IQTEST","accountId":"acc1","status":"PENDING","requestedQuota":5000,"reasonRequested":"이벤트","reasonRejected":"","dateCreated":"2026-05-06T00:00:00.000Z","dateUpdated":"2026-05-06T00:00:00.000Z"}`)
	})

	rootCmd.SetArgs([]string{"quota", "request", "--target", "5000", "--reason", "이벤트 캠페인"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedBody == nil {
		t.Fatal("server did not receive body")
	}
	if capturedBody["quota"] != float64(5000) {
		t.Errorf("body.quota: got %v", capturedBody["quota"])
	}
	if capturedBody["reasonRequested"] != "이벤트 캠페인" {
		t.Errorf("body.reasonRequested: got %v", capturedBody["reasonRequested"])
	}

	out := stdout.String()
	if !strings.Contains(out, "QT01IQTEST") {
		t.Errorf("output missing handleKey: %s", out)
	}
	if !strings.Contains(out, "PENDING") {
		t.Errorf("output missing status: %s", out)
	}
	if !strings.Contains(out, "5,000") {
		t.Errorf("output missing formatted requestedQuota: %s", out)
	}
}

func TestQuotaRequest_MissingTarget(t *testing.T) {
	called := false
	setupQuotaTest(t, func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	})

	rootCmd.SetArgs([]string{"quota", "request", "--reason", "이유"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected validation error when --target missing")
	}
	if !strings.Contains(err.Error(), "요청 한도") {
		t.Errorf("expected 요청 한도 error, got: %v", err)
	}
	if called {
		t.Error("server should not be called when validation fails")
	}
}

func TestQuotaRequest_MissingReason(t *testing.T) {
	called := false
	setupQuotaTest(t, func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	})

	rootCmd.SetArgs([]string{"quota", "request", "--target", "1000"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected validation error when --reason missing")
	}
	if !strings.Contains(err.Error(), "요청 사유") {
		t.Errorf("expected 요청 사유 error, got: %v", err)
	}
	if called {
		t.Error("server should not be called when validation fails")
	}
}

func TestQuotaRequest_EmptyReason(t *testing.T) {
	called := false
	setupQuotaTest(t, func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	})

	rootCmd.SetArgs([]string{"quota", "request", "--target", "1000", "--reason", "   "})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected validation error for whitespace-only reason")
	}
	if !strings.Contains(err.Error(), "요청 사유") {
		t.Errorf("expected 요청 사유 error, got: %v", err)
	}
	if called {
		t.Error("server should not be called when validation fails")
	}
}

func TestQuotaRequest_TargetTooLow(t *testing.T) {
	setupQuotaTest(t, func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("server should not be called")
	})

	rootCmd.SetArgs([]string{"quota", "request", "--target", "49", "--reason", "사유"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "50") || !strings.Contains(err.Error(), "10,000,000") {
		t.Errorf("expected range error mentioning bounds, got: %v", err)
	}
}

func TestQuotaRequest_TargetTooHigh(t *testing.T) {
	setupQuotaTest(t, func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("server should not be called")
	})

	rootCmd.SetArgs([]string{"quota", "request", "--target", "10000001", "--reason", "사유"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestQuotaRequest_ReasonTooLong(t *testing.T) {
	setupQuotaTest(t, func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("server should not be called")
	})

	longReason := strings.Repeat("a", 501)
	rootCmd.SetArgs([]string{"quota", "request", "--target", "1000", "--reason", longReason})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected validation error for reason > 500 runes")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500-char limit error, got: %v", err)
	}
}

func TestQuotaRequest_ReasonRuneCount(t *testing.T) {
	// Verify rune-based length: 500 Korean characters (each 3 bytes UTF-8)
	// must pass; 501 must fail. Guards against accidental len() byte counting.
	stdout, _ := setupQuotaTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"handleKey":"QT","accountId":"a","status":"PENDING","requestedQuota":1000}`)
	})
	five := strings.Repeat("가", 500)
	rootCmd.SetArgs([]string{"quota", "request", "--target", "1000", "--reason", five})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("500 Korean runes should pass, got error: %v", err)
	}
	if !strings.Contains(stdout.String(), "PENDING") {
		t.Error("expected successful request output")
	}
}

func TestQuotaRequest_JSON(t *testing.T) {
	stdout, _ := setupQuotaTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"handleKey":"QT01IQ","status":"PENDING","requestedQuota":1000}`)
	})

	rootCmd.SetArgs([]string{"quota", "request", "--target", "1000", "--reason", "사유", "--json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if parsed["handleKey"] != "QT01IQ" {
		t.Errorf("handleKey: got %v", parsed["handleKey"])
	}
}

func TestQuotaRequest_APIError(t *testing.T) {
	setupQuotaTest(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"errorCode":"InvalidRequestQuota","errorMessage":"요청 한도가 현재 시스템 한도보다 작습니다"}`))
	})

	rootCmd.SetArgs([]string{"quota", "request", "--target", "1000", "--reason", "사유"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for API 400")
	}
	if !strings.Contains(err.Error(), "발송 한도 증가 요청 실패") {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

// ---------- quota list-requests ----------

func TestQuotaListRequests_Success(t *testing.T) {
	stdout, _ := setupQuotaTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/quota/v1/me/system") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("limit"); got != "20" {
			t.Errorf("limit: got %q, want 20 (default)", got)
		}
		_, _ = io.WriteString(w, `{"increaseQuotaList":[{"handleKey":"QTAAA","status":"PENDING","requestedQuota":3000,"reasonRequested":"세일 캠페인","dateCreated":"2026-05-06T10:00:00.000Z"}],"nextKey":""}`)
	})

	rootCmd.SetArgs([]string{"quota", "list-requests"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := stdout.String()
	for _, want := range []string{"REQUEST ID", "QTAAA", "PENDING", "3,000", "세일 캠페인", "2026-05-06"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
}

func TestQuotaListRequests_StatusFilter(t *testing.T) {
	var gotQuery string
	setupQuotaTest(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"increaseQuotaList":[]}`)
	})

	rootCmd.SetArgs([]string{"quota", "list-requests", "--status", "PENDING", "--limit", "5"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotQuery, "status=PENDING") {
		t.Errorf("query missing status=PENDING: %s", gotQuery)
	}
	if !strings.Contains(gotQuery, "limit=5") {
		t.Errorf("query missing limit=5: %s", gotQuery)
	}
}

func TestQuotaListRequests_NextKeyHint(t *testing.T) {
	_, stderr := setupQuotaTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"increaseQuotaList":[{"handleKey":"QT1","status":"APPROVED","requestedQuota":1000,"reasonRequested":"r","dateCreated":"2026-05-01T00:00:00.000Z"}],"nextKey":"NEXTKEY123"}`)
	})

	rootCmd.SetArgs([]string{"quota", "list-requests"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(stderr.String(), "--start-key NEXTKEY123") {
		t.Errorf("stderr should hint at next page: %s", stderr.String())
	}
}

func TestQuotaListRequests_EmptyList(t *testing.T) {
	stdout, stderr := setupQuotaTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"increaseQuotaList":[],"nextKey":""}`)
	})

	rootCmd.SetArgs([]string{"quota", "list-requests"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error on empty list: %v", err)
	}
	if !strings.Contains(stdout.String(), "REQUEST ID") {
		t.Errorf("header should still be printed for empty list: %s", stdout.String())
	}
	if strings.Contains(stderr.String(), "다음 페이지") {
		t.Errorf("nextKey hint should not appear when nextKey is empty: %s", stderr.String())
	}
}

func TestQuotaListRequests_JSON(t *testing.T) {
	stdout, _ := setupQuotaTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"increaseQuotaList":[{"handleKey":"X","status":"PENDING","requestedQuota":500,"reasonRequested":"r","dateCreated":"2026-05-01"}],"nextKey":"K"}`)
	})

	rootCmd.SetArgs([]string{"quota", "list-requests", "--json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if parsed["nextKey"] != "K" {
		t.Errorf("nextKey: got %v", parsed["nextKey"])
	}
}

// ---------- helpers / fuzz ----------

func TestValidateQuotaRequest_Table(t *testing.T) {
	tests := []struct {
		name    string
		target  int
		reason  string
		wantErr string // substring; "" means no error
	}{
		{"missing target", 0, "사유", "요청 한도"},
		{"target below min", 49, "사유", "50"},
		{"target at min", 50, "사유", ""},
		{"target at max", 10_000_000, "사유", ""},
		{"target above max", 10_000_001, "사유", "10,000,000"},
		{"missing reason", 1000, "", "요청 사유"},
		{"whitespace reason", 1000, "   \t\n  ", "요청 사유"},
		{"reason at limit", 1000, strings.Repeat("a", 500), ""},
		{"reason over limit ascii", 1000, strings.Repeat("a", 501), "500"},
		{"reason at limit korean", 1000, strings.Repeat("가", 500), ""},
		{"reason over limit korean", 1000, strings.Repeat("가", 501), "500"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateQuotaRequest(tt.target, tt.reason)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestTruncateReason(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"empty", "", 30, "-"},
		{"short", "hello", 30, "hello"},
		{"exact", strings.Repeat("a", 30), 30, strings.Repeat("a", 30)},
		{"truncated ascii", strings.Repeat("a", 31), 30, strings.Repeat("a", 29) + "…"},
		{"truncated korean", strings.Repeat("가", 31), 30, strings.Repeat("가", 29) + "…"},
		{"max one", "abc", 1, "…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncateReason(tt.in, tt.max); got != tt.want {
				t.Errorf("truncateReason(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
			}
		})
	}
}

func FuzzValidateQuotaRequest(f *testing.F) {
	f.Add(0, "")
	f.Add(50, "사유")
	f.Add(-1, "x")
	f.Add(10_000_000, strings.Repeat("a", 500))
	f.Add(10_000_001, strings.Repeat("가", 501))
	f.Add(int(^uint(0)>>1), "x") // MaxInt
	f.Add(-int(^uint(0)>>1)-1, "") // MinInt

	f.Fuzz(func(t *testing.T, target int, reason string) {
		// Must not panic; must return either nil or a non-nil error.
		_ = validateQuotaRequest(target, reason)
	})
}
