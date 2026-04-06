package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/solapi/solactl/pkg/client"
	"github.com/solapi/solactl/pkg/types"
)

// resetSendFlags resets all send-related flags to zero values.
func resetSendFlags() {
	resetFlags()
	sendFlagTo = ""
	sendFlagFrom = ""
	sendFlagText = ""
	sendFlagScheduled = ""
	sendFlagFile = ""
	sendLMSFlagSubject = ""
	sendMMSFlagImage = ""
	sendMMSFlagSubject = ""
	// ATA
	sendATAFlagPfID = ""
	sendATAFlagTemplateID = ""
	sendATAFlagVariables = ""
	sendATAFlagTitle = ""
	sendATAFlagDisableSms = false
	sendATAFlagButtons = ""
	// BMS
	sendBMSFlagPfID = ""
	sendBMSFlagTemplateID = ""
	sendBMSFlagVariables = ""
	sendBMSFlagFree = false
	sendBMSFlagBubbleType = ""
	sendBMSFlagTargeting = ""
	sendBMSFlagAd = false
	sendBMSFlagAdult = false
	sendBMSFlagImage = ""
	sendBMSFlagButtonName = ""
	sendBMSFlagButtonType = ""
	sendBMSFlagButtonURL = ""
	sendBMSFlagButtons = ""
	// RCS
	sendRCSFlagBrandID = ""
	sendRCSFlagTemplateID = ""
	sendRCSFlagVariables = ""
	sendRCSFlagSubject = ""
	sendRCSFlagImage = ""
	sendRCSFlagMmsType = ""
	sendRCSFlagCopyAllowed = false
}

// setupSendTest creates a test environment with a mock HTTP server.
// The returned cleanup function MUST be called (via t.Cleanup).
func setupSendTest(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("SOLACTL_API_KEY", "testkey")
	t.Setenv("SOLACTL_API_SECRET", "testsecret")

	resetSendFlags()

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
	t.Cleanup(func() {
		clientOverride = nil
		outWriter = nil
		resetSendFlags()
	})

	return ts
}

// captureBuf sets up output capture and returns the buffer.
func captureBuf(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	outWriter = &buf
	return &buf
}

// mockSendResponse returns a realistic send-many/detail response.
func mockSendResponse(total, regSuccess, regFailed int) types.SendResponse {
	return types.SendResponse{
		GroupInfo: types.GroupInfo{
			GroupID:   "G4V20210714152900TESTHASH",
			Status:    "SENDING",
			AccountID: "21000000000000",
			Count: types.GroupCount{
				Total:             total,
				RegisteredSuccess: regSuccess,
				RegisteredFailed:  regFailed,
			},
		},
	}
}

func TestSendSMS_Success(t *testing.T) {
	var mu sync.Mutex
	var captured types.SendRequest

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/messages/v4/send-many/detail") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &captured)
		mu.Unlock()

		resp := mockSendResponse(1, 1, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})

	buf := captureBuf(t)

	rootCmd.SetArgs([]string{"send", "sms", "--to", "01011111111", "--from", "01012345678", "--text", "Hello"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Verify request body
	if len(captured.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(captured.Messages))
	}
	msg := captured.Messages[0]
	if msg.To != "01011111111" {
		t.Errorf("to: got %q, want 01011111111", msg.To)
	}
	if msg.From != "01012345678" {
		t.Errorf("from: got %q, want 01012345678", msg.From)
	}
	if msg.Text != "Hello" {
		t.Errorf("text: got %q, want Hello", msg.Text)
	}

	// Verify agent field is set
	if captured.Agent == nil {
		t.Fatal("agent should not be nil")
	}
	if !strings.HasPrefix(captured.Agent.SDKVersion, "solactl/") {
		t.Errorf("agent sdkVersion: got %q", captured.Agent.SDKVersion)
	}

	// Verify showMessageList
	if captured.ShowMessageList == nil || !*captured.ShowMessageList {
		t.Error("showMessageList should be true")
	}

	// Verify output contains Group ID
	output := buf.String()
	if !strings.Contains(output, "G4V20210714152900TESTHASH") {
		t.Errorf("output should contain group ID, got: %s", output)
	}
}

func TestSendSMS_MultipleRecipients(t *testing.T) {
	var mu sync.Mutex
	var captured types.SendRequest

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &captured)
		mu.Unlock()

		resp := mockSendResponse(2, 2, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})

	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "sms", "--to", "01011111111,01022222222", "--from", "01012345678", "--text", "Hi"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(captured.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(captured.Messages))
	}
	if captured.Messages[0].To != "01011111111" {
		t.Errorf("msg[0].to: got %q", captured.Messages[0].To)
	}
	if captured.Messages[1].To != "01022222222" {
		t.Errorf("msg[1].to: got %q", captured.Messages[1].To)
	}
}

func TestSendSMS_MissingTo(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "sms", "--from", "01012345678", "--text", "Hi"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --to")
	}
	if !strings.Contains(err.Error(), "--to") {
		t.Errorf("error should mention --to: %v", err)
	}
}

func TestSendSMS_MissingText(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "sms", "--to", "01011111111", "--from", "01012345678"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --text")
	}
	if !strings.Contains(err.Error(), "--text") {
		t.Errorf("error should mention --text: %v", err)
	}
}

func TestSendSMS_MissingFrom(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "sms", "--to", "01011111111", "--text", "Hi"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --from")
	}
	if !strings.Contains(err.Error(), "--from") {
		t.Errorf("error should mention --from: %v", err)
	}
}

func TestSendLMS_WithSubject(t *testing.T) {
	var mu sync.Mutex
	var captured types.SendRequest

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &captured)
		mu.Unlock()

		resp := mockSendResponse(1, 1, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})

	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "lms", "--to", "01011111111", "--from", "01012345678", "--text", "긴 메시지 내용입니다", "--subject", "공지사항"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(captured.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(captured.Messages))
	}
	msg := captured.Messages[0]
	if msg.Subject != "공지사항" {
		t.Errorf("subject: got %q, want 공지사항", msg.Subject)
	}
	if msg.Text != "긴 메시지 내용입니다" {
		t.Errorf("text: got %q", msg.Text)
	}
}

func TestSendMMS_UploadAndSend(t *testing.T) {
	var mu sync.Mutex
	var capturedSend types.SendRequest
	var uploadCalled bool

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case strings.HasSuffix(r.URL.Path, "/storage/v1/files"):
			uploadCalled = true

			body, _ := io.ReadAll(r.Body)
			var uploadReq types.UploadFileRequest
			_ = json.Unmarshal(body, &uploadReq)

			if uploadReq.File == "" {
				t.Error("upload file data should not be empty")
			}
			if uploadReq.Type != "MMS" {
				t.Errorf("upload type: got %q, want MMS", uploadReq.Type)
			}

			resp := types.UploadFileResponse{
				FileID: "FILE_ID_12345",
				Name:   "test.jpg",
				URL:    "https://storage.solapi.com/test.jpg",
			}
			data, _ := json.Marshal(resp)
			w.WriteHeader(200)
			_, _ = w.Write(data)

		case strings.HasSuffix(r.URL.Path, "/messages/v4/send-many/detail"):
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &capturedSend)

			resp := mockSendResponse(1, 1, 0)
			data, _ := json.Marshal(resp)
			w.WriteHeader(200)
			_, _ = w.Write(data)

		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	})

	captureBuf(t)

	// Create a temp image file.
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "test.jpg")
	if err := os.WriteFile(imgPath, []byte("fake-image-data"), 0644); err != nil {
		t.Fatalf("write temp image: %v", err)
	}

	rootCmd.SetArgs([]string{"send", "mms", "--to", "01011111111", "--from", "01012345678", "--text", "MMS 테스트", "--image", imgPath})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if !uploadCalled {
		t.Error("upload endpoint should have been called")
	}

	if len(capturedSend.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(capturedSend.Messages))
	}
	msg := capturedSend.Messages[0]
	if msg.ImageID != "FILE_ID_12345" {
		t.Errorf("imageId: got %q, want FILE_ID_12345", msg.ImageID)
	}
}

func TestSendMMS_MissingImage(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "mms", "--to", "01011111111", "--from", "01012345678", "--text", "Test"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --image")
	}
	if !strings.Contains(err.Error(), "--image") {
		t.Errorf("error should mention --image: %v", err)
	}
}

func TestSendSMS_Scheduled(t *testing.T) {
	var mu sync.Mutex
	var captured types.SendRequest

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &captured)
		mu.Unlock()

		resp := mockSendResponse(1, 1, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})

	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "sms", "--to", "01011111111", "--from", "01012345678", "--text", "예약", "--scheduled", "2026-12-31T09:00:00+09:00"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if captured.ScheduledDate != "2026-12-31T09:00:00+09:00" {
		t.Errorf("scheduledDate: got %q, want 2026-12-31T09:00:00+09:00", captured.ScheduledDate)
	}
}

func TestSendSMS_JSONOutput(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		resp := mockSendResponse(1, 1, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})

	buf := captureBuf(t)
	flagJSON = true

	rootCmd.SetArgs([]string{"send", "sms", "--to", "01011111111", "--from", "01012345678", "--text", "Hi", "--json"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// JSON output should be valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}

	// Verify it contains groupInfo
	if _, ok := parsed["groupInfo"]; !ok {
		t.Errorf("JSON output should contain groupInfo, got: %s", output)
	}
}

func TestParseRecipients(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "single number",
			input: "010",
			want:  []string{"010"},
		},
		{
			name:  "two numbers",
			input: "010,020",
			want:  []string{"010", "020"},
		},
		{
			name:  "with spaces",
			input: " 010 , 020 ",
			want:  []string{"010", "020"},
		},
		{
			name:  "trailing comma",
			input: "010,020,",
			want:  []string{"010", "020"},
		},
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "only commas",
			input: ",,,",
			want:  nil,
		},
		{
			name:  "three numbers with various spacing",
			input: "010, 020 ,030",
			want:  []string{"010", "020", "030"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRecipients(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("parseRecipients(%q) = %v (len %d), want %v (len %d)", tt.input, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseRecipients(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestLoadCSVMessages(t *testing.T) {
	t.Run("basic CSV with template", func(t *testing.T) {
		tmpDir := t.TempDir()
		csvPath := filepath.Join(tmpDir, "recipients.csv")
		csvContent := "to,name,amount\n01011111111,홍길동,10000\n01022222222,김철수,20000\n"
		if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
			t.Fatalf("write csv: %v", err)
		}

		msgs, err := loadCSVMessages(csvPath, "01012345678", "{{name}}님, {{amount}}원 입금")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(msgs) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(msgs))
		}

		if msgs[0].To != "01011111111" {
			t.Errorf("msg[0].to: got %q", msgs[0].To)
		}
		if msgs[0].From != "01012345678" {
			t.Errorf("msg[0].from: got %q", msgs[0].From)
		}
		if msgs[0].Text != "홍길동님, 10000원 입금" {
			t.Errorf("msg[0].text: got %q, want '홍길동님, 10000원 입금'", msgs[0].Text)
		}

		if msgs[1].To != "01022222222" {
			t.Errorf("msg[1].to: got %q", msgs[1].To)
		}
		if msgs[1].Text != "김철수님, 20000원 입금" {
			t.Errorf("msg[1].text: got %q", msgs[1].Text)
		}
	})

	t.Run("missing to column", func(t *testing.T) {
		tmpDir := t.TempDir()
		csvPath := filepath.Join(tmpDir, "bad.csv")
		csvContent := "phone,name\n01011111111,홍길동\n"
		if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
			t.Fatalf("write csv: %v", err)
		}

		_, err := loadCSVMessages(csvPath, "01012345678", "Hello")
		if err == nil {
			t.Fatal("expected error for missing to column")
		}
		if !strings.Contains(err.Error(), "to") {
			t.Errorf("error should mention 'to' column: %v", err)
		}
	})

	t.Run("header only (no data rows)", func(t *testing.T) {
		tmpDir := t.TempDir()
		csvPath := filepath.Join(tmpDir, "empty.csv")
		csvContent := "to,name\n"
		if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
			t.Fatalf("write csv: %v", err)
		}

		_, err := loadCSVMessages(csvPath, "01012345678", "Hello")
		if err == nil {
			t.Fatal("expected error for empty CSV")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := loadCSVMessages("/nonexistent/file.csv", "01012345678", "Hello")
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
	})

	t.Run("empty to values skipped", func(t *testing.T) {
		tmpDir := t.TempDir()
		csvPath := filepath.Join(tmpDir, "skip.csv")
		csvContent := "to,name\n01011111111,A\n,B\n01033333333,C\n"
		if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
			t.Fatalf("write csv: %v", err)
		}

		msgs, err := loadCSVMessages(csvPath, "01012345678", "Hi {{name}}")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(msgs) != 2 {
			t.Fatalf("expected 2 messages (empty to skipped), got %d", len(msgs))
		}
		if msgs[0].To != "01011111111" {
			t.Errorf("msg[0].to: got %q", msgs[0].To)
		}
		if msgs[1].To != "01033333333" {
			t.Errorf("msg[1].to: got %q", msgs[1].To)
		}
	})

	t.Run("no template substitution when no placeholders", func(t *testing.T) {
		tmpDir := t.TempDir()
		csvPath := filepath.Join(tmpDir, "plain.csv")
		csvContent := "to,name\n01011111111,Test\n"
		if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
			t.Fatalf("write csv: %v", err)
		}

		msgs, err := loadCSVMessages(csvPath, "01012345678", "Plain text message")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if msgs[0].Text != "Plain text message" {
			t.Errorf("text should be unmodified: got %q", msgs[0].Text)
		}
	})
}

func TestSendSMS_APIError(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"errorCode":"ValidationError","errorMessage":"잘못된 수신번호"}`))
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "sms", "--to", "invalid", "--from", "01012345678", "--text", "Hi"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error from API")
	}
}

func TestSendLMS_MissingFrom(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "lms", "--to", "01011111111", "--text", "Hi"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --from")
	}
	if !strings.Contains(err.Error(), "--from") {
		t.Errorf("error should mention --from: %v", err)
	}
}

func TestSendSMS_FailedMessages(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		resp := types.SendResponse{
			GroupInfo: types.GroupInfo{
				GroupID: "G_FAIL",
				Status:  "SENDING",
				Count: types.GroupCount{
					Total:             2,
					RegisteredSuccess: 1,
					RegisteredFailed:  1,
				},
			},
			FailedMessageList: []types.FailedMessage{
				{
					To:            "01099999999",
					From:          "01012345678",
					StatusCode:    "2000",
					StatusMessage: "수신번호 형식 오류",
				},
			},
		}
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})

	buf := captureBuf(t)

	rootCmd.SetArgs([]string{"send", "sms", "--to", "01011111111,01099999999", "--from", "01012345678", "--text", "Hi"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "실패 메시지") {
		t.Errorf("output should show failed messages section: %s", output)
	}
	if !strings.Contains(output, "01099999999") {
		t.Errorf("output should show failed recipient: %s", output)
	}
}

// ---------------------------------------------------------------------------
// 1. Batching Logic Tests
// ---------------------------------------------------------------------------

func TestSendMessages_SingleBatch(t *testing.T) {
	var mu sync.Mutex
	apiCallCount := 0
	var captured types.SendRequest

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/messages/v4/send-many/detail") {
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			apiCallCount++
			_ = json.Unmarshal(body, &captured)
			mu.Unlock()

			resp := mockSendResponse(3, 3, 0)
			data, _ := json.Marshal(resp)
			w.WriteHeader(200)
			_, _ = w.Write(data)
		} else {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	})

	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "sms",
		"--to", "01011111111,01022222222,01033333333",
		"--from", "01012345678",
		"--text", "Batch test",
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if apiCallCount != 1 {
		t.Errorf("expected exactly 1 API call for 3 messages, got %d", apiCallCount)
	}
	if len(captured.Messages) != 3 {
		t.Fatalf("expected 3 messages in request body, got %d", len(captured.Messages))
	}
	wantTos := []string{"01011111111", "01022222222", "01033333333"}
	for i, want := range wantTos {
		if captured.Messages[i].To != want {
			t.Errorf("messages[%d].to: got %q, want %q", i, captured.Messages[i].To, want)
		}
	}
}

// TestSendMessages_BatchSplit verifies the batching mechanism in sendMessages.
// The production maxBatchSize is 10000 which is impractical to test directly.
// This test validates that the batch-splitting loop in sendMessages correctly
// iterates and calls the API for each batch by code inspection:
//   - The loop `for start := 0; start < len(msgs); start += maxBatchSize`
//     processes messages in chunks of maxBatchSize.
//   - Each iteration sends batch[start:end] where end = min(start+maxBatchSize, len(msgs)).
//   - For N < maxBatchSize messages, exactly 1 API call is made (validated by TestSendMessages_SingleBatch).
//
// To truly test the split without generating 10000+ messages, we use sendMessages
// directly with a small slice and a mock client to count API calls.
func TestSendMessages_BatchSplit(t *testing.T) {
	var mu sync.Mutex
	apiCallCount := 0
	var batchSizes []int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req types.SendRequest
		_ = json.Unmarshal(body, &req)

		mu.Lock()
		apiCallCount++
		batchSizes = append(batchSizes, len(req.Messages))
		mu.Unlock()

		resp := mockSendResponse(len(req.Messages), len(req.Messages), 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	t.Cleanup(ts.Close)

	resetSendFlags()
	captureBuf(t)

	c := &client.Client{
		HTTPClient:      ts.Client(),
		APIKey:          "testkey",
		APISecret:       "testsecret",
		MaxRetries:      0,
		BaseDelay:       time.Millisecond,
		BaseURLOverride: ts.URL,
	}
	clientOverride = c
	t.Cleanup(func() {
		clientOverride = nil
		outWriter = nil
		resetSendFlags()
	})

	// Build 5 messages and call sendMessages directly.
	var msgs []types.Message
	for i := 0; i < 5; i++ {
		msgs = append(msgs, types.Message{
			To:   fmt.Sprintf("0101111%04d", i),
			From: "01012345678",
			Text: "batch test",
		})
	}

	err := sendMessages(c, msgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// With maxBatchSize=10000, 5 messages should result in exactly 1 API call.
	if apiCallCount != 1 {
		t.Errorf("expected 1 API call for 5 messages (maxBatchSize=10000), got %d", apiCallCount)
	}
	if len(batchSizes) != 1 || batchSizes[0] != 5 {
		t.Errorf("expected batch sizes [5], got %v", batchSizes)
	}
}

// ---------------------------------------------------------------------------
// 2. Upload Image Error Paths
// ---------------------------------------------------------------------------

func TestUploadImage_FileNotFound(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called for nonexistent file")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "mms",
		"--to", "01011111111",
		"--from", "01012345678",
		"--text", "Test",
		"--image", "/nonexistent/path.jpg",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent image file")
	}
	if !strings.Contains(err.Error(), "이미지 파일 읽기 실패") {
		t.Errorf("error should contain file read failure message, got: %v", err)
	}
}

func TestUploadImage_StorageAPIError(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/storage/v1/files") {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"errorCode":"InternalError","errorMessage":"서버 오류"}`))
			return
		}
		t.Errorf("unexpected path: %s", r.URL.Path)
		w.WriteHeader(404)
	})
	captureBuf(t)

	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "test.jpg")
	if err := os.WriteFile(imgPath, []byte("fake-image"), 0644); err != nil {
		t.Fatalf("write temp image: %v", err)
	}

	rootCmd.SetArgs([]string{
		"send", "mms",
		"--to", "01011111111",
		"--from", "01012345678",
		"--text", "Test",
		"--image", imgPath,
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for storage API 500")
	}
	if !strings.Contains(err.Error(), "이미지 업로드 실패") {
		t.Errorf("error should contain upload failure message, got: %v", err)
	}
}

func TestUploadImage_EmptyFileID(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/storage/v1/files") {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"fileId":"","name":"test.jpg","url":"https://example.com/test.jpg"}`))
			return
		}
		t.Errorf("unexpected path: %s", r.URL.Path)
		w.WriteHeader(404)
	})
	captureBuf(t)

	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "test.jpg")
	if err := os.WriteFile(imgPath, []byte("fake-image"), 0644); err != nil {
		t.Fatalf("write temp image: %v", err)
	}

	rootCmd.SetArgs([]string{
		"send", "mms",
		"--to", "01011111111",
		"--from", "01012345678",
		"--text", "Test",
		"--image", imgPath,
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for empty fileId")
	}
	if !strings.Contains(err.Error(), "fileId") {
		t.Errorf("error should mention fileId, got: %v", err)
	}
}

func TestUploadImage_MalformedResponse(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/storage/v1/files") {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{not valid json`))
			return
		}
		t.Errorf("unexpected path: %s", r.URL.Path)
		w.WriteHeader(404)
	})
	captureBuf(t)

	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "test.jpg")
	if err := os.WriteFile(imgPath, []byte("fake-image"), 0644); err != nil {
		t.Fatalf("write temp image: %v", err)
	}

	rootCmd.SetArgs([]string{
		"send", "mms",
		"--to", "01011111111",
		"--from", "01012345678",
		"--text", "Test",
		"--image", imgPath,
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for malformed JSON response")
	}
	if !strings.Contains(err.Error(), "업로드 응답 파싱 실패") {
		t.Errorf("error should contain unmarshal failure message, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 3. CSV Edge Cases
// ---------------------------------------------------------------------------

func TestLoadCSVMessages_PlaceholderNoMatch(t *testing.T) {
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "no_match.csv")
	csvContent := "to,name\n01011111111,홍길동\n"
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	msgs, err := loadCSVMessages(csvPath, "01012345678", "hello {{unknown}}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	// {{unknown}} should remain unreplaced because the CSV has no "unknown" column.
	if msgs[0].Text != "hello {{unknown}}" {
		t.Errorf("unreplaced placeholder should remain: got %q, want %q", msgs[0].Text, "hello {{unknown}}")
	}
}

func TestLoadCSVMessages_ShortRow(t *testing.T) {
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "short.csv")
	// Row 2 has fewer columns than headers (only "to", missing "name" and "extra").
	// Go's csv.Reader with FieldsPerRecord=0 will still parse short rows in ReadAll
	// by default unless FieldsPerRecord is set. csv.NewReader defaults FieldsPerRecord=0
	// which means variable-length records. However, the default is actually to expect
	// all rows to match the first record. We need to test what actually happens.
	// The code uses csv.NewReader(f).ReadAll() — by default FieldsPerRecord is set from
	// the first record, so short rows cause a parse error. Let's verify graceful handling.
	csvContent := "to,name,extra\n01011111111,홍길동,A\n01022222222\n"
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	// csv.Reader returns an error on short rows by default (FieldsPerRecord matches first row).
	_, err := loadCSVMessages(csvPath, "01012345678", "hi {{name}}")
	// The code should either handle the short row gracefully or return a CSV read error.
	// Either outcome (error or graceful skip) is acceptable; we verify no panic.
	if err != nil {
		// Should be a CSV read error, not a panic.
		if !strings.Contains(err.Error(), "CSV") {
			t.Errorf("expected CSV-related error, got: %v", err)
		}
	}
}

func TestLoadCSVMessages_QuotedCSV(t *testing.T) {
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "quoted.csv")
	// Quoted field containing comma: "Lee, Kim" should be parsed as one field.
	csvContent := "to,name\n\"01011111111\",\"Lee, Kim\"\n"
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	msgs, err := loadCSVMessages(csvPath, "01012345678", "Hello {{name}}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].To != "01011111111" {
		t.Errorf("to: got %q, want 01011111111", msgs[0].To)
	}
	if msgs[0].Text != "Hello Lee, Kim" {
		t.Errorf("text: got %q, want %q", msgs[0].Text, "Hello Lee, Kim")
	}
}

func TestLoadCSVMessages_CaseInsensitiveToColumn(t *testing.T) {
	tests := []struct {
		name      string
		headerRow string
	}{
		{name: "uppercase TO", headerRow: "TO,name"},
		{name: "mixed case To", headerRow: "To,name"},
		{name: "padded  to ", headerRow: " to ,name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			csvPath := filepath.Join(tmpDir, "case.csv")
			csvContent := tt.headerRow + "\n01011111111,TestName\n"
			if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
				t.Fatalf("write csv: %v", err)
			}

			msgs, err := loadCSVMessages(csvPath, "01012345678", "Hi {{name}}")
			if err != nil {
				t.Fatalf("unexpected error for header %q: %v", tt.headerRow, err)
			}
			if len(msgs) != 1 {
				t.Fatalf("expected 1 message, got %d", len(msgs))
			}
			if msgs[0].To != "01011111111" {
				t.Errorf("to: got %q, want 01011111111", msgs[0].To)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 4. printSendResult Error Handling
// ---------------------------------------------------------------------------

func TestPrintSendResult_UnmarshalError(t *testing.T) {
	resetSendFlags()
	var buf bytes.Buffer
	outWriter = &buf
	t.Cleanup(func() {
		outWriter = nil
		resetSendFlags()
	})

	// Ensure we are NOT in JSON mode so printSendResult tries to unmarshal.
	flagJSON = false

	err := printSendResult(json.RawMessage(`{invalid json`))
	if err == nil {
		t.Fatal("expected unmarshal error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "응답 파싱 실패") {
		t.Errorf("error should contain parse failure message, got: %v", err)
	}
}

func TestPrintSendResult_EmptyFailedList(t *testing.T) {
	resetSendFlags()
	var buf bytes.Buffer
	outWriter = &buf
	t.Cleanup(func() {
		outWriter = nil
		resetSendFlags()
	})

	flagJSON = false

	resp := types.SendResponse{
		GroupInfo: types.GroupInfo{
			GroupID: "G_SUCCESS",
			Status:  "SENDING",
			Count: types.GroupCount{
				Total:             2,
				RegisteredSuccess: 2,
				RegisteredFailed:  0,
			},
		},
		FailedMessageList: []types.FailedMessage{}, // empty
	}
	data, _ := json.Marshal(resp)

	err := printSendResult(json.RawMessage(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "실패 메시지") {
		t.Errorf("output should NOT show failed messages section when list is empty, got: %s", output)
	}
	if !strings.Contains(output, "G_SUCCESS") {
		t.Errorf("output should contain group ID, got: %s", output)
	}
}

// ---------------------------------------------------------------------------
// 5. Flag Validation
// ---------------------------------------------------------------------------

func TestSendSMS_AllFlagsMissing(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "sms"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when no flags provided")
	}
	// The first validation in runSendSMS (after file check) is --to.
	if !strings.Contains(err.Error(), "--to") {
		t.Errorf("error should mention --to as first missing flag, got: %v", err)
	}
}

func TestSendMMS_EmptyImageFlag(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "mms",
		"--to", "01011111111",
		"--from", "01012345678",
		"--text", "Test",
		"--image", "",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for empty --image flag")
	}
	if !strings.Contains(err.Error(), "--image") {
		t.Errorf("error should mention --image, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 6. Fuzz Tests
// ---------------------------------------------------------------------------

func FuzzParseRecipients(f *testing.F) {
	f.Add("010")
	f.Add("010,020")
	f.Add(",,,")
	f.Add("")
	f.Add("a]b[c")

	f.Fuzz(func(t *testing.T, input string) {
		result := parseRecipients(input)
		if result == nil {
			// nil is acceptable (means no valid recipients)
			return
		}
		// result must be a slice; verify each element is non-empty and trimmed.
		for i, r := range result {
			if r == "" {
				t.Errorf("parseRecipients(%q)[%d] is empty string", input, i)
			}
			if r != strings.TrimSpace(r) {
				t.Errorf("parseRecipients(%q)[%d] = %q is not trimmed", input, i, r)
			}
		}
	})
}

func FuzzLoadCSVMessages(f *testing.F) {
	f.Add([]byte("to,name\n01011111111,Test\n"))
	f.Add([]byte(""))
	f.Add([]byte("to\n010\n"))
	f.Add([]byte{0xff, 0xfe, 0x00, 0x01, 0x80, 0x7f})

	f.Fuzz(func(t *testing.T, data []byte) {
		tmpDir := t.TempDir()
		csvPath := filepath.Join(tmpDir, "fuzz.csv")
		if err := os.WriteFile(csvPath, data, 0644); err != nil {
			t.Fatalf("write temp file: %v", err)
		}

		// Must not panic regardless of input.
		_, _ = loadCSVMessages(csvPath, "01012345678", "hello {{name}}")
	})
}

// ---------------------------------------------------------------------------
// 7. Boundary: Batch Size
// ---------------------------------------------------------------------------

func TestSendMessages_ZeroMessages(t *testing.T) {
	apiCallCount := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCallCount++
		resp := mockSendResponse(0, 0, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	t.Cleanup(ts.Close)

	resetSendFlags()
	captureBuf(t)

	c := &client.Client{
		HTTPClient:      ts.Client(),
		APIKey:          "testkey",
		APISecret:       "testsecret",
		MaxRetries:      0,
		BaseDelay:       time.Millisecond,
		BaseURLOverride: ts.URL,
	}
	clientOverride = c
	t.Cleanup(func() {
		clientOverride = nil
		outWriter = nil
		resetSendFlags()
	})

	err := sendMessages(c, []types.Message{})
	if err != nil {
		t.Fatalf("sendMessages with empty slice should not error, got: %v", err)
	}
	if apiCallCount != 0 {
		t.Errorf("expected 0 API calls for empty message slice, got %d", apiCallCount)
	}
}

func TestSendMessages_ExactlyMaxBatch(t *testing.T) {
	origMax := maxBatchSize
	maxBatchSize = 5
	t.Cleanup(func() { maxBatchSize = origMax })

	var mu sync.Mutex
	apiCallCount := 0
	var batchSizes []int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req types.SendRequest
		_ = json.Unmarshal(body, &req)

		mu.Lock()
		apiCallCount++
		batchSizes = append(batchSizes, len(req.Messages))
		mu.Unlock()

		resp := mockSendResponse(len(req.Messages), len(req.Messages), 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	t.Cleanup(ts.Close)

	resetSendFlags()
	captureBuf(t)

	c := &client.Client{
		HTTPClient:      ts.Client(),
		APIKey:          "testkey",
		APISecret:       "testsecret",
		MaxRetries:      0,
		BaseDelay:       time.Millisecond,
		BaseURLOverride: ts.URL,
	}
	clientOverride = c
	t.Cleanup(func() {
		clientOverride = nil
		outWriter = nil
		resetSendFlags()
	})

	// Exactly maxBatchSize (5) messages: should result in exactly 1 API call.
	var msgs []types.Message
	for i := 0; i < 5; i++ {
		msgs = append(msgs, types.Message{
			To:   fmt.Sprintf("0101111%04d", i),
			From: "01012345678",
			Text: "exact batch test",
		})
	}

	err := sendMessages(c, msgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if apiCallCount != 1 {
		t.Errorf("expected 1 API call for exactly maxBatchSize messages, got %d", apiCallCount)
	}
	if len(batchSizes) != 1 || batchSizes[0] != 5 {
		t.Errorf("expected batch sizes [5], got %v", batchSizes)
	}
}

func TestSendMessages_MultipleBatches(t *testing.T) {
	origMax := maxBatchSize
	maxBatchSize = 3
	t.Cleanup(func() { maxBatchSize = origMax })

	var mu sync.Mutex
	apiCallCount := 0
	var batchSizes []int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req types.SendRequest
		_ = json.Unmarshal(body, &req)

		mu.Lock()
		apiCallCount++
		batchSizes = append(batchSizes, len(req.Messages))
		mu.Unlock()

		resp := mockSendResponse(len(req.Messages), len(req.Messages), 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	t.Cleanup(ts.Close)

	resetSendFlags()
	captureBuf(t)

	c := &client.Client{
		HTTPClient:      ts.Client(),
		APIKey:          "testkey",
		APISecret:       "testsecret",
		MaxRetries:      0,
		BaseDelay:       time.Millisecond,
		BaseURLOverride: ts.URL,
	}
	clientOverride = c
	t.Cleanup(func() {
		clientOverride = nil
		outWriter = nil
		resetSendFlags()
	})

	// 7 messages with maxBatchSize=3 should yield 3 API calls: 3 + 3 + 1.
	var msgs []types.Message
	for i := 0; i < 7; i++ {
		msgs = append(msgs, types.Message{
			To:   fmt.Sprintf("0101111%04d", i),
			From: "01012345678",
			Text: "multi batch test",
		})
	}

	err := sendMessages(c, msgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if apiCallCount != 3 {
		t.Errorf("expected 3 API calls for 7 messages with maxBatchSize=3, got %d", apiCallCount)
	}
	wantBatches := []int{3, 3, 1}
	if len(batchSizes) != len(wantBatches) {
		t.Fatalf("expected %d batches, got %d: %v", len(wantBatches), len(batchSizes), batchSizes)
	}
	for i, want := range wantBatches {
		if batchSizes[i] != want {
			t.Errorf("batch[%d] size: got %d, want %d", i, batchSizes[i], want)
		}
	}
}

// ---------------------------------------------------------------------------
// 8. CSV Edge Cases (additional)
// ---------------------------------------------------------------------------

func TestLoadCSVMessages_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "empty.csv")
	// Write a completely empty file (0 bytes).
	if err := os.WriteFile(csvPath, []byte{}, 0644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	_, err := loadCSVMessages(csvPath, "01012345678", "Hello")
	if err == nil {
		t.Fatal("expected error for empty (0-byte) CSV file")
	}
}

func TestLoadCSVMessages_ToColumnAfterOthers(t *testing.T) {
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "to_third.csv")
	// "to" is the 3rd column, not the 1st.
	csvContent := "name,age,to\nJohn,30,01012345678\nJane,25,01099998888\n"
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	msgs, err := loadCSVMessages(csvPath, "01000000000", "Hi {{name}}, age {{age}}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].To != "01012345678" {
		t.Errorf("msg[0].to: got %q, want 01012345678", msgs[0].To)
	}
	if msgs[0].Text != "Hi John, age 30" {
		t.Errorf("msg[0].text: got %q, want %q", msgs[0].Text, "Hi John, age 30")
	}
	if msgs[1].To != "01099998888" {
		t.Errorf("msg[1].to: got %q, want 01099998888", msgs[1].To)
	}
	if msgs[1].Text != "Hi Jane, age 25" {
		t.Errorf("msg[1].text: got %q, want %q", msgs[1].Text, "Hi Jane, age 25")
	}
}

func TestLoadCSVMessages_NoToColumn(t *testing.T) {
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "noto.csv")
	csvContent := "name,phone\nAlice,01012345678\nBob,01099998888\n"
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	_, err := loadCSVMessages(csvPath, "01000000000", "Hello {{name}}")
	if err == nil {
		t.Fatal("expected error for CSV with no 'to' column")
	}
	if !strings.Contains(err.Error(), "to") {
		t.Errorf("error should mention missing 'to' column, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ATA (Kakao Alimtalk) Tests
// ---------------------------------------------------------------------------

func TestSendATA_Success(t *testing.T) {
	var mu sync.Mutex
	var captured types.SendRequest

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &captured)
		mu.Unlock()

		resp := mockSendResponse(1, 1, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})

	buf := captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "ata",
		"--to", "01011111111",
		"--pfid", "KA01PF001",
		"--template-id", "KA01TP001",
		"--variables", `{"#{이름}":"홍길동"}`,
		"--text", "안녕하세요 홍길동님",
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(captured.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(captured.Messages))
	}
	msg := captured.Messages[0]
	if msg.KakaoOptions == nil {
		t.Fatal("kakaoOptions should not be nil")
	}
	if msg.KakaoOptions.PfID != "KA01PF001" {
		t.Errorf("pfId: got %q", msg.KakaoOptions.PfID)
	}
	if msg.KakaoOptions.TemplateID != "KA01TP001" {
		t.Errorf("templateId: got %q", msg.KakaoOptions.TemplateID)
	}
	if msg.KakaoOptions.Variables["#{이름}"] != "홍길동" {
		t.Errorf("variables: got %v", msg.KakaoOptions.Variables)
	}
	if !strings.Contains(buf.String(), "G4V20210714152900TESTHASH") {
		t.Errorf("output should contain group ID")
	}
}

func TestSendATA_WithButtons(t *testing.T) {
	var mu sync.Mutex
	var captured types.SendRequest

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &captured)
		mu.Unlock()

		resp := mockSendResponse(1, 1, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "ata",
		"--to", "01011111111",
		"--pfid", "KA01PF001",
		"--template-id", "KA01TP001",
		"--text", "버튼 테스트",
		"--buttons", `[{"buttonType":"WL","buttonName":"바로가기","linkMo":"https://example.com"}]`,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(captured.Messages[0].KakaoOptions.Buttons) != 1 {
		t.Fatalf("expected 1 button, got %d", len(captured.Messages[0].KakaoOptions.Buttons))
	}
	btn := captured.Messages[0].KakaoOptions.Buttons[0]
	if btn.ButtonType != "WL" || btn.ButtonName != "바로가기" {
		t.Errorf("button: got type=%q name=%q", btn.ButtonType, btn.ButtonName)
	}
}

func TestSendATA_MissingPfID(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "ata", "--to", "01011111111", "--template-id", "T1"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --pfid")
	}
	if !strings.Contains(err.Error(), "--pfid") {
		t.Errorf("error should mention --pfid: %v", err)
	}
}

func TestSendATA_MissingTemplateID(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "ata", "--to", "01011111111", "--pfid", "PF1"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --template-id")
	}
	if !strings.Contains(err.Error(), "--template-id") {
		t.Errorf("error should mention --template-id: %v", err)
	}
}

func TestSendATA_MissingTo(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "ata", "--pfid", "PF1", "--template-id", "T1"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --to")
	}
	if !strings.Contains(err.Error(), "--to") {
		t.Errorf("error should mention --to: %v", err)
	}
}

func TestSendATA_OptionalFrom(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		resp := mockSendResponse(1, 1, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	captureBuf(t)

	// from is not provided — should succeed for Kakao types
	rootCmd.SetArgs([]string{
		"send", "ata",
		"--to", "01011111111",
		"--pfid", "KA01PF001",
		"--template-id", "KA01TP001",
		"--text", "테스트",
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("ATA should not require --from: %v", err)
	}
}

func TestSendATA_InvalidVariablesJSON(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "ata",
		"--to", "01011111111",
		"--pfid", "PF1",
		"--template-id", "T1",
		"--text", "테스트",
		"--variables", "{invalid json}",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "변수 JSON 파싱 실패") {
		t.Errorf("error should mention JSON parse failure: %v", err)
	}
}

func TestSendATA_InvalidButtonsJSON(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "ata",
		"--to", "01011111111",
		"--pfid", "PF1",
		"--template-id", "T1",
		"--text", "테스트",
		"--buttons", "[{bad}]",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid buttons JSON")
	}
	if !strings.Contains(err.Error(), "버튼 JSON 파싱 실패") {
		t.Errorf("error should mention button parse failure: %v", err)
	}
}

func TestSendATA_TooManyButtons(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	buttons := `[{"buttonType":"WL"},{"buttonType":"WL"},{"buttonType":"WL"},{"buttonType":"WL"},{"buttonType":"WL"},{"buttonType":"WL"}]`
	rootCmd.SetArgs([]string{
		"send", "ata",
		"--to", "01011111111",
		"--pfid", "PF1",
		"--template-id", "T1",
		"--text", "테스트",
		"--buttons", buttons,
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for >5 buttons")
	}
	if !strings.Contains(err.Error(), "최대 5개") {
		t.Errorf("error should mention max 5 buttons: %v", err)
	}
}

func TestSendATA_WithDisableSms(t *testing.T) {
	var mu sync.Mutex
	var captured types.SendRequest

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &captured)
		mu.Unlock()

		resp := mockSendResponse(1, 1, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "ata",
		"--to", "01011111111",
		"--pfid", "PF1",
		"--template-id", "T1",
		"--text", "테스트",
		"--disable-sms",
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	ko := captured.Messages[0].KakaoOptions
	if ko.DisableSms == nil || !*ko.DisableSms {
		t.Error("disableSms should be true")
	}
}

func TestSendATA_WithTitle(t *testing.T) {
	var mu sync.Mutex
	var captured types.SendRequest

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &captured)
		mu.Unlock()

		resp := mockSendResponse(1, 1, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "ata",
		"--to", "01011111111",
		"--pfid", "PF1",
		"--template-id", "T1",
		"--text", "테스트",
		"--title", "강조 제목",
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if captured.Messages[0].KakaoOptions.Title != "강조 제목" {
		t.Errorf("title: got %q", captured.Messages[0].KakaoOptions.Title)
	}
}

func TestSendATA_MultipleRecipients(t *testing.T) {
	var mu sync.Mutex
	var captured types.SendRequest

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &captured)
		mu.Unlock()

		resp := mockSendResponse(2, 2, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "ata",
		"--to", "01011111111,01022222222",
		"--pfid", "PF1",
		"--template-id", "T1",
		"--text", "테스트",
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(captured.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(captured.Messages))
	}
	for _, msg := range captured.Messages {
		if msg.KakaoOptions == nil {
			t.Error("each message should have kakaoOptions")
		}
	}
}

func TestSendATA_JSONOutput(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		resp := mockSendResponse(1, 1, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})

	buf := captureBuf(t)
	flagJSON = true

	rootCmd.SetArgs([]string{
		"send", "ata",
		"--to", "01011111111",
		"--pfid", "PF1",
		"--template-id", "T1",
		"--text", "테스트",
		"--json",
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
}

func TestSendATA_APIError(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"errorCode":"ValidationError","errorMessage":"유효하지 않은 템플릿"}`))
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "ata",
		"--to", "01011111111",
		"--pfid", "PF1",
		"--template-id", "INVALID",
		"--text", "테스트",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error from API")
	}
}

func TestSendATA_CSVBulk(t *testing.T) {
	var mu sync.Mutex
	var captured types.SendRequest

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &captured)
		mu.Unlock()

		resp := mockSendResponse(2, 2, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	captureBuf(t)

	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "ata.csv")
	csvContent := "to,name\n01011111111,홍길동\n01022222222,김영희\n"
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	rootCmd.SetArgs([]string{
		"send", "ata",
		"--file", csvPath,
		"--pfid", "PF1",
		"--template-id", "T1",
		"--text", "안녕하세요 {{name}}님",
		"--to", "dummy", // required by parseRecipients but overridden by CSV
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(captured.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(captured.Messages))
	}
	for _, msg := range captured.Messages {
		if msg.KakaoOptions == nil || msg.KakaoOptions.PfID != "PF1" {
			t.Errorf("CSV messages should have kakaoOptions with pfId=PF1")
		}
	}
}

// ---------------------------------------------------------------------------
// BMS (Kakao Business Message) Tests
// ---------------------------------------------------------------------------

func TestSendBMS_TemplateSuccess(t *testing.T) {
	var mu sync.Mutex
	var captured types.SendRequest

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &captured)
		mu.Unlock()

		resp := mockSendResponse(1, 1, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "bms",
		"--to", "01011111111",
		"--pfid", "PF1",
		"--template-id", "KA01BPTPL001",
		"--targeting", "I",
		"--variables", `{"#{이름}":"홍길동"}`,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	msg := captured.Messages[0]
	if msg.KakaoOptions == nil {
		t.Fatal("kakaoOptions should not be nil")
	}
	if msg.KakaoOptions.TemplateID != "KA01BPTPL001" {
		t.Errorf("templateId: got %q", msg.KakaoOptions.TemplateID)
	}
	if msg.KakaoOptions.BMS == nil || msg.KakaoOptions.BMS.Targeting != "I" {
		t.Error("BMS targeting should be I")
	}
}

func TestSendBMS_TemplateMissingPfID(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "bms", "--to", "010", "--template-id", "T1", "--targeting", "I"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --pfid")
	}
	if !strings.Contains(err.Error(), "--pfid") {
		t.Errorf("error should mention --pfid: %v", err)
	}
}

func TestSendBMS_TemplateMissingTemplateID(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "bms", "--to", "010", "--pfid", "PF1", "--targeting", "I"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --template-id")
	}
	if !strings.Contains(err.Error(), "--template-id") {
		t.Errorf("error should mention --template-id: %v", err)
	}
}

func TestSendBMS_FreeSuccess(t *testing.T) {
	var mu sync.Mutex
	var captured types.SendRequest

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &captured)
		mu.Unlock()

		resp := mockSendResponse(1, 1, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "bms",
		"--free",
		"--to", "01011111111",
		"--pfid", "PF1",
		"--bubble-type", "TEXT",
		"--targeting", "M",
		"--text", "자유형 메시지",
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	msg := captured.Messages[0]
	if msg.KakaoOptions == nil || msg.KakaoOptions.BMS == nil {
		t.Fatal("BMS options should not be nil")
	}
	if msg.KakaoOptions.BMS.ChatBubbleType != "TEXT" {
		t.Errorf("chatBubbleType: got %q", msg.KakaoOptions.BMS.ChatBubbleType)
	}
	if msg.KakaoOptions.BMS.Targeting != "M" {
		t.Errorf("targeting: got %q", msg.KakaoOptions.BMS.Targeting)
	}
}

func TestSendBMS_FreeMissingBubbleType(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "bms", "--free", "--to", "010", "--pfid", "PF1", "--targeting", "I"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --bubble-type")
	}
	if !strings.Contains(err.Error(), "--bubble-type") {
		t.Errorf("error should mention --bubble-type: %v", err)
	}
}

func TestSendBMS_FreeMissingTargeting(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "bms", "--to", "010", "--pfid", "PF1"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --targeting")
	}
	if !strings.Contains(err.Error(), "--targeting") {
		t.Errorf("error should mention --targeting: %v", err)
	}
}

func TestSendBMS_FreeInvalidBubbleType(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "bms", "--free",
		"--to", "010",
		"--pfid", "PF1",
		"--bubble-type", "INVALID",
		"--targeting", "I",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid bubble type")
	}
	if !strings.Contains(err.Error(), "유효하지 않은 chatBubbleType") {
		t.Errorf("error should mention invalid chatBubbleType: %v", err)
	}
}

func TestSendBMS_FreeInvalidTargeting(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "bms", "--to", "010", "--pfid", "PF1", "--targeting", "X"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid targeting")
	}
	if !strings.Contains(err.Error(), "유효하지 않은 targeting") {
		t.Errorf("error should mention invalid targeting: %v", err)
	}
}

func TestSendBMS_FreeWithImage(t *testing.T) {
	var mu sync.Mutex
	var captured types.SendRequest
	var uploadType string

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case strings.HasSuffix(r.URL.Path, "/storage/v1/files"):
			body, _ := io.ReadAll(r.Body)
			var req types.UploadFileRequest
			_ = json.Unmarshal(body, &req)
			uploadType = req.Type

			resp := types.UploadFileResponse{FileID: "BMS_IMG_001", Name: "bms.jpg"}
			data, _ := json.Marshal(resp)
			w.WriteHeader(200)
			_, _ = w.Write(data)

		case strings.HasSuffix(r.URL.Path, "/messages/v4/send-many/detail"):
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &captured)

			resp := mockSendResponse(1, 1, 0)
			data, _ := json.Marshal(resp)
			w.WriteHeader(200)
			_, _ = w.Write(data)

		default:
			w.WriteHeader(404)
		}
	})
	captureBuf(t)

	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "bms.jpg")
	_ = os.WriteFile(imgPath, []byte("fake-bms-image"), 0644)

	rootCmd.SetArgs([]string{
		"send", "bms", "--free",
		"--to", "01011111111",
		"--pfid", "PF1",
		"--bubble-type", "IMAGE",
		"--targeting", "I",
		"--text", "이미지 메시지",
		"--image", imgPath,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if uploadType != "KAKAO" {
		t.Errorf("upload type should be KAKAO, got %q", uploadType)
	}
	if captured.Messages[0].KakaoOptions.ImageID != "BMS_IMG_001" {
		t.Errorf("imageId: got %q", captured.Messages[0].KakaoOptions.ImageID)
	}
}

func TestSendBMS_FreeWithAdFlag(t *testing.T) {
	var mu sync.Mutex
	var captured types.SendRequest

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &captured)
		mu.Unlock()

		resp := mockSendResponse(1, 1, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "bms", "--free",
		"--to", "01011111111",
		"--pfid", "PF1",
		"--bubble-type", "TEXT",
		"--targeting", "M",
		"--text", "광고 메시지",
		"--ad",
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	ko := captured.Messages[0].KakaoOptions
	if ko.AdFlag == nil || !*ko.AdFlag {
		t.Error("adFlag should be true")
	}
}

func TestSendBMS_FreeWithSingleButton(t *testing.T) {
	var mu sync.Mutex
	var captured types.SendRequest

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &captured)
		mu.Unlock()

		resp := mockSendResponse(1, 1, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "bms", "--free",
		"--to", "01011111111",
		"--pfid", "PF1",
		"--bubble-type", "TEXT",
		"--targeting", "I",
		"--text", "버튼 테스트",
		"--button-name", "자세히 보기",
		"--button-type", "WL",
		"--button-url", "https://example.com",
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	buttons := captured.Messages[0].KakaoOptions.Buttons
	if len(buttons) != 1 {
		t.Fatalf("expected 1 button, got %d", len(buttons))
	}
	if buttons[0].ButtonName != "자세히 보기" {
		t.Errorf("button name: got %q", buttons[0].ButtonName)
	}
}

func TestSendBMS_OptionalFrom(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		resp := mockSendResponse(1, 1, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "bms",
		"--to", "01011111111",
		"--pfid", "PF1",
		"--template-id", "T1",
		"--targeting", "I",
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("BMS should not require --from: %v", err)
	}
}

func TestSendBMS_FreeAdultFlag(t *testing.T) {
	var mu sync.Mutex
	var captured types.SendRequest

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &captured)
		mu.Unlock()

		resp := mockSendResponse(1, 1, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "bms", "--free",
		"--to", "01011111111",
		"--pfid", "PF1",
		"--bubble-type", "TEXT",
		"--targeting", "I",
		"--text", "성인 컨텐츠",
		"--adult",
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	bms := captured.Messages[0].KakaoOptions.BMS
	if bms.Adult == nil || !*bms.Adult {
		t.Error("adult should be true")
	}
}

// ---------------------------------------------------------------------------
// RCS Tests
// ---------------------------------------------------------------------------

func TestSendRCS_Success(t *testing.T) {
	var mu sync.Mutex
	var captured types.SendRequest

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &captured)
		mu.Unlock()

		resp := mockSendResponse(1, 1, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "rcs",
		"--to", "01011111111",
		"--from", "01012345678",
		"--text", "RCS 메시지",
		"--brand-id", "BRAND001",
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	msg := captured.Messages[0]
	if msg.RCSOptions == nil {
		t.Fatal("rcsOptions should not be nil")
	}
	if msg.RCSOptions.BrandID != "BRAND001" {
		t.Errorf("brandId: got %q", msg.RCSOptions.BrandID)
	}
	if msg.Text != "RCS 메시지" {
		t.Errorf("text: got %q", msg.Text)
	}
}

func TestSendRCS_WithTemplate(t *testing.T) {
	var mu sync.Mutex
	var captured types.SendRequest

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &captured)
		mu.Unlock()

		resp := mockSendResponse(1, 1, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "rcs",
		"--to", "01011111111",
		"--from", "01012345678",
		"--brand-id", "BRAND001",
		"--template-id", "RCSTPL001",
		"--variables", `{"key":"value"}`,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	rcs := captured.Messages[0].RCSOptions
	if rcs.TemplateID != "RCSTPL001" {
		t.Errorf("templateId: got %q", rcs.TemplateID)
	}
	if rcs.Variables["key"] != "value" {
		t.Errorf("variables: got %v", rcs.Variables)
	}
}

func TestSendRCS_MissingBrandID(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "rcs", "--to", "010", "--from", "011", "--text", "Hi"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --brand-id")
	}
	if !strings.Contains(err.Error(), "--brand-id") {
		t.Errorf("error should mention --brand-id: %v", err)
	}
}

func TestSendRCS_MissingFrom(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "rcs", "--to", "010", "--text", "Hi", "--brand-id", "B1"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --from")
	}
	if !strings.Contains(err.Error(), "--from") {
		t.Errorf("error should mention --from: %v", err)
	}
}

func TestSendRCS_MissingTo(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "rcs", "--from", "011", "--text", "Hi", "--brand-id", "B1"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --to")
	}
	if !strings.Contains(err.Error(), "--to") {
		t.Errorf("error should mention --to: %v", err)
	}
}

func TestSendRCS_TextRequiredWithoutTemplate(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "rcs", "--to", "010", "--from", "011", "--brand-id", "B1"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --text without --template-id")
	}
	if !strings.Contains(err.Error(), "--text") {
		t.Errorf("error should mention --text: %v", err)
	}
}

func TestSendRCS_TextOptionalWithTemplate(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		resp := mockSendResponse(1, 1, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "rcs",
		"--to", "01011111111",
		"--from", "01012345678",
		"--brand-id", "B1",
		"--template-id", "TPL1",
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("RCS with template should not require --text: %v", err)
	}
}

func TestSendRCS_WithSubject(t *testing.T) {
	var mu sync.Mutex
	var captured types.SendRequest

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &captured)
		mu.Unlock()

		resp := mockSendResponse(1, 1, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "rcs",
		"--to", "01011111111",
		"--from", "01012345678",
		"--text", "RCS 메시지",
		"--brand-id", "B1",
		"--subject", "RCS 제목",
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if captured.Messages[0].Subject != "RCS 제목" {
		t.Errorf("subject: got %q", captured.Messages[0].Subject)
	}
}

func TestSendRCS_WithMmsType(t *testing.T) {
	var mu sync.Mutex
	var captured types.SendRequest

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &captured)
		mu.Unlock()

		resp := mockSendResponse(1, 1, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "rcs",
		"--to", "01011111111",
		"--from", "01012345678",
		"--text", "RCS MMS",
		"--brand-id", "B1",
		"--mms-type", "M3",
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if captured.Messages[0].RCSOptions.MmsType != "M3" {
		t.Errorf("mmsType: got %q", captured.Messages[0].RCSOptions.MmsType)
	}
}

func TestSendRCS_CopyAllowed(t *testing.T) {
	var mu sync.Mutex
	var captured types.SendRequest

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &captured)
		mu.Unlock()

		resp := mockSendResponse(1, 1, 0)
		data, _ := json.Marshal(resp)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "rcs",
		"--to", "01011111111",
		"--from", "01012345678",
		"--text", "Copy test",
		"--brand-id", "B1",
		"--copy-allowed",
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	rcs := captured.Messages[0].RCSOptions
	if rcs.CopyAllowed == nil || !*rcs.CopyAllowed {
		t.Error("copyAllowed should be true")
	}
}

func TestSendRCS_InvalidVariablesJSON(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "rcs",
		"--to", "010",
		"--from", "011",
		"--text", "Hi",
		"--brand-id", "B1",
		"--variables", "{bad json}",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "변수 JSON 파싱 실패") {
		t.Errorf("error should mention JSON parse failure: %v", err)
	}
}

func TestSendRCS_WithImage(t *testing.T) {
	var mu sync.Mutex
	var captured types.SendRequest
	var uploadType string

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case strings.HasSuffix(r.URL.Path, "/storage/v1/files"):
			body, _ := io.ReadAll(r.Body)
			var req types.UploadFileRequest
			_ = json.Unmarshal(body, &req)
			uploadType = req.Type

			resp := types.UploadFileResponse{FileID: "RCS_IMG_001"}
			data, _ := json.Marshal(resp)
			w.WriteHeader(200)
			_, _ = w.Write(data)

		case strings.HasSuffix(r.URL.Path, "/messages/v4/send-many/detail"):
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &captured)

			resp := mockSendResponse(1, 1, 0)
			data, _ := json.Marshal(resp)
			w.WriteHeader(200)
			_, _ = w.Write(data)

		default:
			w.WriteHeader(404)
		}
	})
	captureBuf(t)

	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "rcs.jpg")
	_ = os.WriteFile(imgPath, []byte("fake-rcs-image"), 0644)

	rootCmd.SetArgs([]string{
		"send", "rcs",
		"--to", "01011111111",
		"--from", "01012345678",
		"--text", "RCS with image",
		"--brand-id", "B1",
		"--image", imgPath,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if uploadType != "RCS" {
		t.Errorf("upload type should be RCS, got %q", uploadType)
	}
	if captured.Messages[0].ImageID != "RCS_IMG_001" {
		t.Errorf("imageId: got %q", captured.Messages[0].ImageID)
	}
}

// ---------------------------------------------------------------------------
// Shared Helper Tests
// ---------------------------------------------------------------------------

func TestParseVariables_ValidJSON(t *testing.T) {
	result, err := parseVariables(`{"#{이름}":"홍길동","#{금액}":"10000"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["#{이름}"] != "홍길동" {
		t.Errorf("got %q for #{이름}", result["#{이름}"])
	}
	if result["#{금액}"] != "10000" {
		t.Errorf("got %q for #{금액}", result["#{금액}"])
	}
}

func TestParseVariables_EmptyString(t *testing.T) {
	result, err := parseVariables("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

func TestParseVariables_InvalidJSON(t *testing.T) {
	_, err := parseVariables("{not valid}")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "변수 JSON 파싱 실패") {
		t.Errorf("error should mention parse failure: %v", err)
	}
}

func TestParseVariables_NonStringValues(t *testing.T) {
	_, err := parseVariables(`{"key":123}`)
	if err == nil {
		t.Fatal("expected error for non-string values")
	}
	if !strings.Contains(err.Error(), "문자열") {
		t.Errorf("error should mention string requirement: %v", err)
	}
}

func TestParseKakaoButtons_ValidJSON(t *testing.T) {
	result, err := parseKakaoButtons(`[{"buttonType":"WL","buttonName":"링크","linkMo":"https://example.com"}]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 button, got %d", len(result))
	}
	if result[0].ButtonType != "WL" {
		t.Errorf("buttonType: got %q", result[0].ButtonType)
	}
}

func FuzzParseVariables(f *testing.F) {
	f.Add(`{"key":"value"}`)
	f.Add(`{}`)
	f.Add(`{"a":"b","c":"d"}`)
	f.Add(`{invalid`)
	f.Add(``)
	f.Add(`null`)
	f.Add(`[]`)

	f.Fuzz(func(t *testing.T, input string) {
		// Must not panic
		_, _ = parseVariables(input)
	})
}

func FuzzParseKakaoButtons(f *testing.F) {
	f.Add(`[{"buttonType":"WL"}]`)
	f.Add(`[]`)
	f.Add(`{bad}`)
	f.Add(``)
	f.Add(`null`)

	f.Fuzz(func(t *testing.T, input string) {
		// Must not panic
		_, _ = parseKakaoButtons(input)
	})
}

// ---------------------------------------------------------------------------
// Additional Validation Branch Tests (from review feedback)
// ---------------------------------------------------------------------------

func TestSendATA_MissingText(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{"send", "ata", "--to", "010", "--pfid", "PF1", "--template-id", "T1"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --text")
	}
	if !strings.Contains(err.Error(), "--text") {
		t.Errorf("error should mention --text: %v", err)
	}
}

func TestSendBMS_FreeMissingText(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "bms", "--free",
		"--to", "010",
		"--pfid", "PF1",
		"--bubble-type", "TEXT",
		"--targeting", "I",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --text in free mode")
	}
	if !strings.Contains(err.Error(), "--text") {
		t.Errorf("error should mention --text: %v", err)
	}
}

func TestSendBMS_FreeImageRequired(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	for _, bubbleType := range []string{"IMAGE", "WIDE"} {
		t.Run(bubbleType, func(t *testing.T) {
			rootCmd.SetArgs([]string{
				"send", "bms", "--free",
				"--to", "010",
				"--pfid", "PF1",
				"--bubble-type", bubbleType,
				"--targeting", "I",
				"--text", "테스트",
			})
			err := rootCmd.Execute()
			if err == nil {
				t.Fatalf("expected error for missing --image with %s", bubbleType)
			}
			if !strings.Contains(err.Error(), "--image") {
				t.Errorf("error should mention --image: %v", err)
			}
		})
	}
}

func TestSendBMS_ButtonsFlagConflict(t *testing.T) {
	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	captureBuf(t)

	rootCmd.SetArgs([]string{
		"send", "bms", "--free",
		"--to", "010",
		"--pfid", "PF1",
		"--bubble-type", "TEXT",
		"--targeting", "I",
		"--text", "테스트",
		"--buttons", `[{"buttonType":"WL"}]`,
		"--button-name", "중복",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for conflicting button flags")
	}
	if !strings.Contains(err.Error(), "--buttons") || !strings.Contains(err.Error(), "--button-name") {
		t.Errorf("error should mention flag conflict: %v", err)
	}
}

func TestSendBMS_FreeWithWideImage(t *testing.T) {
	var mu sync.Mutex
	var uploadType string

	setupSendTest(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case strings.HasSuffix(r.URL.Path, "/storage/v1/files"):
			body, _ := io.ReadAll(r.Body)
			var req types.UploadFileRequest
			_ = json.Unmarshal(body, &req)
			uploadType = req.Type

			resp := types.UploadFileResponse{FileID: "WIDE_IMG_001"}
			data, _ := json.Marshal(resp)
			w.WriteHeader(200)
			_, _ = w.Write(data)

		case strings.HasSuffix(r.URL.Path, "/messages/v4/send-many/detail"):
			resp := mockSendResponse(1, 1, 0)
			data, _ := json.Marshal(resp)
			w.WriteHeader(200)
			_, _ = w.Write(data)

		default:
			w.WriteHeader(404)
		}
	})
	captureBuf(t)

	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "wide.jpg")
	_ = os.WriteFile(imgPath, []byte("fake-wide-image"), 0644)

	rootCmd.SetArgs([]string{
		"send", "bms", "--free",
		"--to", "01011111111",
		"--pfid", "PF1",
		"--bubble-type", "WIDE",
		"--targeting", "I",
		"--text", "와이드 메시지",
		"--image", imgPath,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if uploadType != "BMS_WIDE" {
		t.Errorf("WIDE bubble type should upload with BMS_WIDE type, got %q", uploadType)
	}
}
