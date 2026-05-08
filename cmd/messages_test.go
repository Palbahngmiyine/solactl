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

func resetMessagesFlags() {
	resetFlags()
	msgListFlagLimit = 20
	msgListFlagStartDate = ""
	msgListFlagEndDate = ""
	msgListFlagType = ""
	msgListFlagStartKey = ""
}

func setupMessagesTest(t *testing.T, handler http.HandlerFunc) *bytes.Buffer {
	t.Helper()
	resetMessagesFlags()

	// Reset cobra flags to allow re-registration
	messagesListCmd.ResetFlags()
	messagesListCmd.Flags().IntVar(&msgListFlagLimit, "limit", 20, "조회 건수")
	messagesListCmd.Flags().StringVar(&msgListFlagStartDate, "start-date", "", "시작 날짜")
	messagesListCmd.Flags().StringVar(&msgListFlagEndDate, "end-date", "", "종료 날짜")
	messagesListCmd.Flags().StringVar(&msgListFlagType, "type", "", "메시지 타입")
	messagesListCmd.Flags().StringVar(&msgListFlagStartKey, "start-key", "", "페이지네이션 시작 키")

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

	var buf bytes.Buffer
	outWriter = &buf

	t.Cleanup(func() {
		clientOverride = nil
		outWriter = nil
		resetMessagesFlags()
	})

	return &buf
}

func TestMessagesList_Success(t *testing.T) {
	buf := setupMessagesTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/messages/v4/list") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := `{
			"messageList": {
				"MSG001": {"to":"01011111111","from":"01012345678","type":"SMS","statusCode":"2000","dateCreated":"2026-04-06T10:00:00"},
				"MSG002": {"to":"01022222222","from":"01012345678","type":"LMS","statusCode":"2000","dateCreated":"2026-04-06T11:00:00"}
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, resp)
	})

	rootCmd.SetArgs([]string{"messages", "list"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "MESSAGE ID") {
		t.Errorf("output should contain table headers: %s", output)
	}
	// At least one message should appear
	if !strings.Contains(output, "SMS") && !strings.Contains(output, "LMS") {
		t.Errorf("output should contain message types: %s", output)
	}
}

func TestMessagesList_Empty(t *testing.T) {
	buf := setupMessagesTest(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"messageList":{}}`)
	})

	rootCmd.SetArgs([]string{"messages", "list"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "MESSAGE ID") {
		t.Errorf("output should show table headers even when empty: %s", output)
	}
}

func TestMessagesList_WithPagination(t *testing.T) {
	buf := setupMessagesTest(t, func(w http.ResponseWriter, r *http.Request) {
		resp := `{
			"messageList": {"MSG001": {"to":"010","type":"SMS","statusCode":"2000","dateCreated":"2026-04-06T10:00:00"}},
			"nextKey": "abc123"
		}`
		_, _ = io.WriteString(w, resp)
	})

	rootCmd.SetArgs([]string{"messages", "list"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "다음 페이지") {
		t.Errorf("output should contain pagination hint: %s", output)
	}
	if !strings.Contains(output, "abc123") {
		t.Errorf("output should contain next key: %s", output)
	}
}

func TestMessagesList_NoPagination(t *testing.T) {
	buf := setupMessagesTest(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"messageList":{"MSG001":{"to":"010","type":"SMS","statusCode":"2000","dateCreated":"2026-04-06T10:00:00"}}}`)
	})

	rootCmd.SetArgs([]string{"messages", "list"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(buf.String(), "다음 페이지") {
		t.Error("output should not contain pagination hint when no nextKey")
	}
}

func TestMessagesList_JSON(t *testing.T) {
	buf := setupMessagesTest(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"messageList":{"MSG001":{"to":"010","type":"SMS","statusCode":"2000","dateCreated":"2026-04-06T10:00:00"}}}`)
	})
	flagJSON = true

	rootCmd.SetArgs([]string{"messages", "list", "--json"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
}

func TestMessagesList_WithAllFlags(t *testing.T) {
	var capturedQuery string

	setupMessagesTest(t, func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"messageList":{}}`)
	})

	rootCmd.SetArgs([]string{
		"messages", "list",
		"--limit", "5",
		"--start-date", "2026-04-01",
		"--end-date", "2026-04-06",
		"--type", "SMS",
		"--start-key", "KEY123",
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(capturedQuery, "limit=5") {
		t.Errorf("query should contain limit=5: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "startDate=2026-04-01") {
		t.Errorf("query should contain startDate: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "endDate=2026-04-06") {
		t.Errorf("query should contain endDate: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "type=SMS") {
		t.Errorf("query should contain type=SMS: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "startKey=KEY123") {
		t.Errorf("query should contain startKey: %s", capturedQuery)
	}
}

func TestMessagesList_DefaultLimit(t *testing.T) {
	var capturedQuery string

	setupMessagesTest(t, func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"messageList":{}}`)
	})

	rootCmd.SetArgs([]string{"messages", "list"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(capturedQuery, "limit=20") {
		t.Errorf("default limit should be 20: %s", capturedQuery)
	}
}

func TestMessagesList_APIError(t *testing.T) {
	setupMessagesTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"errorCode":"InternalError","errorMessage":"서버 오류"}`))
	})

	rootCmd.SetArgs([]string{"messages", "list"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for API 500")
	}
}

func TestMessagesList_MalformedJSON(t *testing.T) {
	setupMessagesTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{bad json}`))
	})

	rootCmd.SetArgs([]string{"messages", "list"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "응답 파싱 실패") {
		t.Errorf("error should mention parse failure: %v", err)
	}
}
