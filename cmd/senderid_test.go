package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/solapi/solactl/pkg/client"
)

// resetSenderIDFlags resets all senderid-specific package-level flag variables.
func resetSenderIDFlags() {
	flagAll = false
	flagStatus = ""
	flagYes = false
	stdinReader = nil
}

// setupSenderIDTest creates a test client pointing at the given httptest server
// and configures the cmd package globals for isolated test execution.
func setupSenderIDTest(t *testing.T, server *httptest.Server) *bytes.Buffer {
	t.Helper()

	resetFlags()
	resetSenderIDFlags()

	// Reset cobra command flag states so MarkFlagRequired works across tests.
	senderidUpdateCmd.ResetFlags()
	senderidUpdateCmd.Flags().StringVar(&flagStatus, "status", "", "변경할 상태 (ACTIVE|INACTIVE)")
	_ = senderidUpdateCmd.MarkFlagRequired("status")

	senderidDeleteCmd.ResetFlags()
	senderidDeleteCmd.Flags().BoolVarP(&flagYes, "yes", "y", false, "확인 없이 삭제")

	senderidListCmd.ResetFlags()
	senderidListCmd.Flags().BoolVar(&flagAll, "all", false, "모든 발신번호 조회 (비활성 포함)")

	t.Cleanup(func() {
		clientOverride = nil
		outWriter = nil
		resetFlags()
		resetSenderIDFlags()
	})

	c := client.New("testkey", "testsecret")
	c.BaseURLOverride = server.URL
	c.MaxRetries = 0
	clientOverride = c

	var buf bytes.Buffer
	outWriter = &buf
	return &buf
}

// ── List Active ─────────────────────────────────────────────────────────

func TestSenderIDList_Active(t *testing.T) {
	resp := `{
		"accountId": "acc1",
		"limit": 10,
		"senderIds": [
			{"phoneNumber":"01012345678","status":"ACTIVE","method":"ARS","expireAt":"2025-12-31T23:59:59"},
			{"phoneNumber":"01099998888","status":"INACTIVE","method":"","expireAt":""},
			{"phoneNumber":"01087654321","status":"ACTIVE","method":"DOCUMENT","expireAt":""}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/senderid/v1/numbers") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, resp)
	}))
	defer server.Close()

	buf := setupSenderIDTest(t, server)

	rootCmd.SetArgs([]string{"senderid", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Verify table headers
	if !strings.Contains(output, "PHONE NUMBER") {
		t.Errorf("expected PHONE NUMBER header, got:\n%s", output)
	}
	if !strings.Contains(output, "STATUS") {
		t.Errorf("expected STATUS header, got:\n%s", output)
	}

	// Verify ACTIVE entries are shown
	if !strings.Contains(output, "01012345678") {
		t.Errorf("expected phone 01012345678 in output, got:\n%s", output)
	}
	if !strings.Contains(output, "01087654321") {
		t.Errorf("expected phone 01087654321 in output, got:\n%s", output)
	}
	if !strings.Contains(output, "ARS") {
		t.Errorf("expected method ARS in output, got:\n%s", output)
	}
	if !strings.Contains(output, "2025-12-31") {
		t.Errorf("expected expire date 2025-12-31 in output, got:\n%s", output)
	}

	// Verify INACTIVE entry is filtered out
	if strings.Contains(output, "01099998888") {
		t.Errorf("INACTIVE phone 01099998888 should be filtered out, got:\n%s", output)
	}
	if strings.Contains(output, "INACTIVE") {
		t.Errorf("INACTIVE status should not appear in default list, got:\n%s", output)
	}
}

// ── List All ────────────────────────────────────────────────────────────

func TestSenderIDList_All(t *testing.T) {
	allResp := `{
		"accountId": "acc123",
		"limit": 10,
		"senderIds": [
			{"phoneNumber":"01012345678","status":"ACTIVE","method":"ARS","expireAt":"2025-12-31T23:59:59"},
			{"phoneNumber":"01099998888","status":"INACTIVE","method":"","expireAt":""}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		// --all uses /senderid/v1/numbers (no /active suffix)
		if !strings.HasSuffix(r.URL.Path, "/senderid/v1/numbers") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, allResp)
	}))
	defer server.Close()

	buf := setupSenderIDTest(t, server)

	rootCmd.SetArgs([]string{"senderid", "list", "--all"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "01012345678") {
		t.Errorf("expected active phone in output, got:\n%s", output)
	}
	if !strings.Contains(output, "01099998888") {
		t.Errorf("expected inactive phone in output, got:\n%s", output)
	}
	if !strings.Contains(output, "INACTIVE") {
		t.Errorf("expected INACTIVE status in output, got:\n%s", output)
	}
}

// ── List JSON Mode ──────────────────────────────────────────────────────

func TestSenderIDList_JSON(t *testing.T) {
	resp := `{"accountId":"acc1","limit":10,"senderIds":[
		{"phoneNumber":"01012345678","status":"ACTIVE","method":"ARS","expireAt":"2025-12-31"},
		{"phoneNumber":"01099998888","status":"INACTIVE","method":"","expireAt":""}
	]}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, resp)
	}))
	defer server.Close()

	buf := setupSenderIDTest(t, server)
	flagJSON = true

	rootCmd.SetArgs([]string{"senderid", "list", "--json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Should be valid JSON
	var parsed json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &parsed); err != nil {
		t.Errorf("output is not valid JSON: %v\noutput:\n%s", err, output)
	}

	// Should NOT contain table headers
	if strings.Contains(output, "PHONE NUMBER") {
		t.Errorf("JSON mode should not contain table headers, got:\n%s", output)
	}

	// Should contain the ACTIVE phone number
	if !strings.Contains(output, "01012345678") {
		t.Errorf("expected ACTIVE phone in JSON output, got:\n%s", output)
	}

	// Should NOT contain the INACTIVE phone number (filtered out)
	if strings.Contains(output, "01099998888") {
		t.Errorf("INACTIVE phone should be filtered from default JSON output, got:\n%s", output)
	}
}

// ── Update Success ──────────────────────────────────────────────────────

func TestSenderIDUpdate_Success(t *testing.T) {
	var receivedMethod string
	var receivedPath string
	var receivedBody map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path

		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedBody)

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"phoneNumber":"01012345678","status":"ACTIVE"}`)
	}))
	defer server.Close()

	buf := setupSenderIDTest(t, server)

	rootCmd.SetArgs([]string{"senderid", "update", "01012345678", "--status", "ACTIVE"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify request
	if receivedMethod != http.MethodPut {
		t.Errorf("expected PUT, got %s", receivedMethod)
	}
	if !strings.HasSuffix(receivedPath, "/senderid/v1/numbers/01012345678") {
		t.Errorf("unexpected path: %s", receivedPath)
	}
	if receivedBody["status"] != "ACTIVE" {
		t.Errorf("expected status ACTIVE in body, got: %v", receivedBody)
	}

	// Verify output
	output := buf.String()
	if !strings.Contains(output, "01012345678") {
		t.Errorf("expected phone in output, got:\n%s", output)
	}
	if !strings.Contains(output, "ACTIVE") {
		t.Errorf("expected ACTIVE in output, got:\n%s", output)
	}
}

// ── Update Missing Status ───────────────────────────────────────────────

func TestSenderIDUpdate_MissingStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called when --status is missing")
	}))
	defer server.Close()

	_ = setupSenderIDTest(t, server)

	rootCmd.SetArgs([]string{"senderid", "update", "01012345678"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --status flag")
	}
	if !strings.Contains(err.Error(), "status") {
		t.Errorf("error should mention 'status', got: %v", err)
	}
}

// ── Update Missing Phone ────────────────────────────────────────────────

func TestSenderIDUpdate_MissingPhone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called when phone arg is missing")
	}))
	defer server.Close()

	_ = setupSenderIDTest(t, server)

	rootCmd.SetArgs([]string{"senderid", "update", "--status", "ACTIVE"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing phone argument")
	}
}

// ── Delete With Yes Flag ────────────────────────────────────────────────

func TestSenderIDDelete_WithYesFlag(t *testing.T) {
	var receivedMethod string
	var receivedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()

	buf := setupSenderIDTest(t, server)

	rootCmd.SetArgs([]string{"senderid", "delete", "01012345678", "--yes"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify DELETE was sent
	if receivedMethod != http.MethodDelete {
		t.Errorf("expected DELETE, got %s", receivedMethod)
	}
	if !strings.HasSuffix(receivedPath, "/senderid/v1/numbers/01012345678") {
		t.Errorf("unexpected path: %s", receivedPath)
	}

	// Verify success output
	output := buf.String()
	if !strings.Contains(output, "01012345678") {
		t.Errorf("expected phone in output, got:\n%s", output)
	}
	if !strings.Contains(output, "삭제") {
		t.Errorf("expected delete confirmation in output, got:\n%s", output)
	}
}

// ── Delete Missing Phone ────────────────────────────────────────────────

func TestSenderIDDelete_MissingPhone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called when phone arg is missing")
	}))
	defer server.Close()

	_ = setupSenderIDTest(t, server)

	rootCmd.SetArgs([]string{"senderid", "delete", "--yes"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing phone argument")
	}
}

// ── Delete Confirmation Flow ────────────────────────────────────────────

func TestSenderIDDelete_ConfirmY(t *testing.T) {
	var deleteCalled atomic.Bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled.Store(true)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()

	buf := setupSenderIDTest(t, server)
	stdinReader = bufio.NewReader(strings.NewReader("y\n"))

	rootCmd.SetArgs([]string{"senderid", "delete", "01012345678"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !deleteCalled.Load() {
		t.Error("expected DELETE request to be made when user confirms with 'y'")
	}
	output := buf.String()
	if !strings.Contains(output, "삭제되었습니다") {
		t.Errorf("expected delete success message, got:\n%s", output)
	}
}

func TestSenderIDDelete_ConfirmYes(t *testing.T) {
	var deleteCalled atomic.Bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled.Store(true)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()

	buf := setupSenderIDTest(t, server)
	stdinReader = bufio.NewReader(strings.NewReader("yes\n"))

	rootCmd.SetArgs([]string{"senderid", "delete", "01012345678"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !deleteCalled.Load() {
		t.Error("expected DELETE request to be made when user confirms with 'yes'")
	}
	output := buf.String()
	if !strings.Contains(output, "삭제되었습니다") {
		t.Errorf("expected delete success message, got:\n%s", output)
	}
}

func TestSenderIDDelete_ConfirmUpperY(t *testing.T) {
	var deleteCalled atomic.Bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled.Store(true)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()

	_ = setupSenderIDTest(t, server)
	stdinReader = bufio.NewReader(strings.NewReader("Y\n"))

	rootCmd.SetArgs([]string{"senderid", "delete", "01012345678"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !deleteCalled.Load() {
		t.Error("expected DELETE request to be made when user confirms with 'Y' (uppercase)")
	}
}

func TestSenderIDDelete_RejectN(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			t.Fatal("DELETE should NOT be called when user rejects with 'n'")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()

	buf := setupSenderIDTest(t, server)
	stdinReader = bufio.NewReader(strings.NewReader("n\n"))

	rootCmd.SetArgs([]string{"senderid", "delete", "01012345678"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "취소") {
		t.Errorf("expected cancellation message containing '취소', got:\n%s", output)
	}
}

func TestSenderIDDelete_RejectEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			t.Fatal("DELETE should NOT be called when user presses enter without input")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()

	buf := setupSenderIDTest(t, server)
	stdinReader = bufio.NewReader(strings.NewReader("\n"))

	rootCmd.SetArgs([]string{"senderid", "delete", "01012345678"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "취소") {
		t.Errorf("expected cancellation message containing '취소', got:\n%s", output)
	}
}

func TestSenderIDDelete_RejectEOF(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			t.Fatal("DELETE should NOT be called on EOF")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()

	_ = setupSenderIDTest(t, server)
	// Empty reader produces EOF immediately
	stdinReader = bufio.NewReader(strings.NewReader(""))

	rootCmd.SetArgs([]string{"senderid", "delete", "01012345678"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error on EOF input")
	}
	if !strings.Contains(err.Error(), "입력을 읽을 수 없습니다") {
		t.Errorf("expected read error message, got: %v", err)
	}
}

// ── Error Paths ─────────────────────────────────────────────────────────

func TestSenderIDList_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"errorCode":"InternalError","errorMessage":"서버 오류"}`)
	}))
	defer server.Close()

	_ = setupSenderIDTest(t, server)

	rootCmd.SetArgs([]string{"senderid", "list"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when API returns 500")
	}
	if !strings.Contains(err.Error(), "발신번호 조회 실패") {
		t.Errorf("expected '발신번호 조회 실패' in error, got: %v", err)
	}
}

func TestSenderIDList_EmptyActive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"accountId":"acc1","limit":10,"senderIds":[]}`)
	}))
	defer server.Close()

	buf := setupSenderIDTest(t, server)

	rootCmd.SetArgs([]string{"senderid", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Table should contain headers but no data rows
	if !strings.Contains(output, "PHONE NUMBER") {
		t.Errorf("expected PHONE NUMBER header even for empty list, got:\n%s", output)
	}
	// Should not contain any phone number data
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Skip header and separator lines
		if strings.Contains(trimmed, "PHONE NUMBER") || strings.HasPrefix(trimmed, "─") || strings.HasPrefix(trimmed, "-") {
			continue
		}
		// Any remaining line with digits would indicate a data row
		if strings.ContainsAny(trimmed, "0123456789") {
			t.Errorf("expected no data rows for empty list, but found: %s", trimmed)
		}
	}
}

func TestSenderIDList_EmptyAll(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"accountId":"acc1","limit":10,"senderIds":[]}`)
	}))
	defer server.Close()

	buf := setupSenderIDTest(t, server)

	rootCmd.SetArgs([]string{"senderid", "list", "--all"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "PHONE NUMBER") {
		t.Errorf("expected PHONE NUMBER header even for empty --all list, got:\n%s", output)
	}
}

func TestSenderIDUpdate_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"errorCode":"ValidationError","errorMessage":"잘못된 상태"}`)
	}))
	defer server.Close()

	_ = setupSenderIDTest(t, server)

	rootCmd.SetArgs([]string{"senderid", "update", "01012345678", "--status", "INVALID"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when API returns 400")
	}
	if !strings.Contains(err.Error(), "발신번호 상태 변경 실패") {
		t.Errorf("expected '발신번호 상태 변경 실패' in error, got: %v", err)
	}
}

func TestSenderIDDelete_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"errorCode":"InternalError","errorMessage":"삭제 실패"}`)
	}))
	defer server.Close()

	_ = setupSenderIDTest(t, server)

	rootCmd.SetArgs([]string{"senderid", "delete", "01012345678", "--yes"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when API returns 500")
	}
	if !strings.Contains(err.Error(), "발신번호 삭제 실패") {
		t.Errorf("expected '발신번호 삭제 실패' in error, got: %v", err)
	}
}

// ── Unmarshal Error Handling ────────────────────────────────────────────

func TestSenderIDList_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{this is not valid json!!!`)
	}))
	defer server.Close()

	_ = setupSenderIDTest(t, server)

	rootCmd.SetArgs([]string{"senderid", "list"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for malformed JSON response")
	}
	if !strings.Contains(err.Error(), "응답 파싱 실패") {
		t.Errorf("expected '응답 파싱 실패' in error, got: %v", err)
	}
}

// ── Table Formatting Edge Cases ─────────────────────────────────────────

func TestSenderIDList_AllFieldsEmpty(t *testing.T) {
	// SenderID with all empty optional fields — use --all to show entries without ACTIVE status
	resp := `{"accountId":"acc1","limit":10,"senderIds":[{"phoneNumber":"01000000000","status":"","method":"","expireAt":""}]}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, resp)
	}))
	defer server.Close()

	buf := setupSenderIDTest(t, server)

	rootCmd.SetArgs([]string{"senderid", "list", "--all"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "01000000000") {
		t.Errorf("expected phone number in output, got:\n%s", output)
	}

	// Empty method and expireAt should show "-"
	// Count occurrences of "-" that are not part of table borders
	// The phone number row should have "-" for status, method, and expireAt
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "01000000000") {
			// This data row should contain "-" for empty fields
			dashCount := strings.Count(line, "-")
			// At minimum, method and expireAt should be "-" (status too since it's empty)
			if dashCount < 3 {
				t.Errorf("expected at least 3 dashes for empty fields in data row, got %d in:\n%s", dashCount, line)
			}
			break
		}
	}
}
