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

func resetBalanceFlags() {
	resetFlags()
}

func setupBalanceTest(t *testing.T, handler http.HandlerFunc) *bytes.Buffer {
	t.Helper()
	resetBalanceFlags()

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
		resetBalanceFlags()
	})

	return &buf
}

func TestBalance_Success(t *testing.T) {
	buf := setupBalanceTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/cash/v1/balance") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"balance":12500,"point":3000}`)
	})

	rootCmd.SetArgs([]string{"balance"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "12,500원") {
		t.Errorf("output should contain formatted balance: %s", output)
	}
	if !strings.Contains(output, "3,000P") {
		t.Errorf("output should contain formatted point: %s", output)
	}
}

func TestBalance_Zero(t *testing.T) {
	buf := setupBalanceTest(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"balance":0,"point":0}`)
	})

	rootCmd.SetArgs([]string{"balance"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "0원") {
		t.Errorf("output should contain 0원: %s", output)
	}
	if !strings.Contains(output, "0P") {
		t.Errorf("output should contain 0P: %s", output)
	}
}

func TestBalance_LargeNumbers(t *testing.T) {
	buf := setupBalanceTest(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"balance":1000000,"point":99999}`)
	})

	rootCmd.SetArgs([]string{"balance"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "1,000,000원") {
		t.Errorf("output should contain 1,000,000원: %s", output)
	}
	if !strings.Contains(output, "99,999P") {
		t.Errorf("output should contain 99,999P: %s", output)
	}
}

func TestBalance_NegativeBalance(t *testing.T) {
	buf := setupBalanceTest(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"balance":-1500,"point":0}`)
	})

	rootCmd.SetArgs([]string{"balance"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "-1,500원") {
		t.Errorf("output should contain -1,500원: %s", output)
	}
}

func TestBalance_JSON(t *testing.T) {
	buf := setupBalanceTest(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"balance":12500,"point":3000}`)
	})
	flagJSON = true

	rootCmd.SetArgs([]string{"balance", "--json"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if parsed["balance"] != float64(12500) {
		t.Errorf("balance: got %v", parsed["balance"])
	}
}

func TestBalance_APIError(t *testing.T) {
	setupBalanceTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"errorCode":"Unauthorized","errorMessage":"인증 실패"}`))
	})

	rootCmd.SetArgs([]string{"balance"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for API 401")
	}
}

func TestBalance_MalformedJSON(t *testing.T) {
	setupBalanceTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{not json}`))
	})

	rootCmd.SetArgs([]string{"balance"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "응답 파싱 실패") {
		t.Errorf("error should mention parse failure: %v", err)
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{999, "999"},
		{1000, "1,000"},
		{12500, "12,500"},
		{1000000, "1,000,000"},
		{-1500, "-1,500"},
		{-999, "-999"},
		{100, "100"},
		{99999999, "99,999,999"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatNumber(tt.input)
			if got != tt.want {
				t.Errorf("formatNumber(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func FuzzFormatNumber(f *testing.F) {
	f.Add(0)
	f.Add(1)
	f.Add(-1)
	f.Add(1000)
	f.Add(999999999)
	f.Add(-999999999)

	f.Fuzz(func(t *testing.T, n int) {
		result := formatNumber(n)
		if result == "" {
			t.Error("formatNumber should never return empty string")
		}
	})
}
