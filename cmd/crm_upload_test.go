package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/solapi/solactl/pkg/client"
)

func setupCRMUploadTest(t *testing.T, handler http.HandlerFunc) *bytes.Buffer {
	t.Helper()
	resetFlags()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SOLACTL_API_KEY", "TESTKEY")
	t.Setenv("SOLACTL_API_SECRET", "TESTSECRET")

	apiSrv := httptest.NewServer(handler)
	c := &client.Client{
		HTTPClient:      apiSrv.Client(),
		APIKey:          "TESTKEY",
		APISecret:       "TESTSECRET",
		MaxRetries:      0,
		BaseDelay:       time.Millisecond,
		BaseURLOverride: apiSrv.URL,
	}
	clientOverride = c

	var outBuf bytes.Buffer
	outWriter = &outBuf
	t.Cleanup(func() {
		clientOverride = nil
		outWriter = nil
		errWriter = nil
		apiSrv.Close()
		resetFlags()
	})
	return &outBuf
}

func writeUploadFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeSizedUploadFile(t *testing.T, name string, size int64) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(size); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCRMUploadRecordsImportExcel_SendsMultipartFieldsAndQuery(t *testing.T) {
	filePath := writeUploadFile(t, "records.xlsx", []byte("PK\x03\x04excel"))
	stdout := setupCRMUploadTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s", r.Method)
		}
		if r.URL.Path != "/crm-core/v1/records/import/excel" {
			t.Errorf("path: got %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("skipAutomation"); got != "true" {
			t.Errorf("skipAutomation: got %q", got)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		checks := map[string]string{
			"entityId":       "ENxxx",
			"sheetName":      "Contacts",
			"hasHeader":      "false",
			"columnMappings": `{"0":"__name__"}`,
			"linkConfigs":    `[{"entityId":"ENlink"}]`,
		}
		for key, want := range checks {
			if got := r.FormValue(key); got != want {
				t.Errorf("field %s: got %q, want %q", key, got, want)
			}
		}
		f, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile: %v", err)
		}
		defer func() { _ = f.Close() }()
		if header.Filename != "records.xlsx" {
			t.Errorf("filename: got %q", header.Filename)
		}
		if got := header.Header.Get("Content-Type"); got != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
			t.Errorf("file content-type: got %q", got)
		}
		_, _ = io.WriteString(w, `{"jobId":"JOB1"}`)
	})

	rootCmd.SetArgs([]string{
		"crm", "records", "import-excel",
		"--file", filePath,
		"--entity-id", "ENxxx",
		"--sheet-name", "Contacts",
		"--has-header=false",
		"--column-mappings", `{"0":"__name__"}`,
		"--link-configs", `[{"entityId":"ENlink"}]`,
		"--skip-automation",
		"--format", "json",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]string
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &parsed); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if parsed["jobId"] != "JOB1" {
		t.Errorf("jobId: got %q", parsed["jobId"])
	}
}

func TestCRMUploadFormsUploadImage_SendsPurposeQueryAndImageMime(t *testing.T) {
	filePath := writeUploadFile(t, "cover.png", []byte("\x89PNG\r\n\x1a\nimage"))
	setupCRMUploadTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/crm-core/v1/forms/FORM1/images" {
			t.Errorf("path: got %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("purpose"); got != "cover" {
			t.Errorf("purpose: got %q", got)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		f, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile: %v", err)
		}
		defer func() { _ = f.Close() }()
		if got := header.Header.Get("Content-Type"); got != "image/png" {
			t.Errorf("file content-type: got %q", got)
		}
		_, _ = io.WriteString(w, `{"fileId":"FILE1"}`)
	})

	rootCmd.SetArgs([]string{"crm", "forms", "upload-image", "FORM1", "--file", filePath, "--purpose", "cover", "--format", "json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCRMUploadDocumentTemplateAttachment_AllowsUnfilteredFileExtension(t *testing.T) {
	filePath := writeUploadFile(t, "archive.zip", []byte("PK\x03\x04zip"))
	setupCRMUploadTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/crm-core/v1/document-templates/TPL1/versions/VER1/attachments" {
			t.Errorf("path: got %s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		f, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile: %v", err)
		}
		defer func() { _ = f.Close() }()
		if header.Filename != "archive.zip" {
			t.Errorf("filename: got %q", header.Filename)
		}
		_, _ = io.WriteString(w, `{"fileId":"FILE1"}`)
	})

	rootCmd.SetArgs([]string{"crm", "document-templates", "upload-version-attachment", "TPL1", "VER1", "--file", filePath, "--format", "json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewCRMUploadClient_PublicDoesNotRequireCredentials(t *testing.T) {
	resetFlags()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SOLACTL_API_KEY", "")
	t.Setenv("SOLACTL_API_SECRET", "")
	clientOverride = nil
	t.Cleanup(resetFlags)

	c, err := newCRMUploadClient(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !c.SkipAuthorization {
		t.Fatal("public upload client should not attach Authorization")
	}
}

func TestCRMUploadRecordsUploadAttachment_RejectsEmptyFileBeforeRequest(t *testing.T) {
	filePath := writeUploadFile(t, "empty.pdf", nil)
	var called bool
	setupCRMUploadTest(t, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = io.WriteString(w, `{}`)
	})

	rootCmd.SetArgs([]string{"crm", "records", "upload-attachment", "REC1", "--file", filePath})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("want error for empty file")
	}
	if !strings.Contains(err.Error(), "비어") {
		t.Errorf("error should explain empty file: %v", err)
	}
	if called {
		t.Fatal("server should not be called when local file validation fails")
	}
}

func TestCRMUploadRecordsImportExcel_RejectsInvalidJSONBeforeRequest(t *testing.T) {
	filePath := writeUploadFile(t, "records.xlsx", []byte("PK\x03\x04excel"))
	var called bool
	setupCRMUploadTest(t, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = io.WriteString(w, `{}`)
	})

	rootCmd.SetArgs([]string{"crm", "records", "import-excel", "--file", filePath, "--entity-id", "ENxxx", "--column-mappings", `{not-json}`})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("want invalid JSON error")
	}
	if !strings.Contains(err.Error(), "--column-mappings") || !strings.Contains(err.Error(), "JSON") {
		t.Errorf("error should identify invalid JSON flag: %v", err)
	}
	if called {
		t.Fatal("server should not be called when JSON flag validation fails")
	}
}

func TestCRMUploadAgentUploadFile_RejectsUnsupportedExtension(t *testing.T) {
	filePath := writeUploadFile(t, "script.exe", []byte("MZ"))
	var called bool
	setupCRMUploadTest(t, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = io.WriteString(w, `{}`)
	})

	rootCmd.SetArgs([]string{"crm", "agent", "upload-file", "--file", filePath})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("want unsupported extension error")
	}
	if !strings.Contains(err.Error(), "지원하지 않습니다") {
		t.Errorf("error should explain unsupported extension: %v", err)
	}
	if called {
		t.Fatal("server should not be called when extension validation fails")
	}
}

func TestValidateCRMUploadFile_BoundariesAndFailures(t *testing.T) {
	constraint := crmUploadConstraint{
		label:         "테스트 파일",
		maxBytes:      3,
		maxBytesByExt: nil,
		extensions:    extSet(".txt"),
	}

	atMax := writeUploadFile(t, "ok.txt", []byte("123"))
	if err := validateCRMUploadFile(atMax, constraint); err != nil {
		t.Fatalf("file at max size should pass: %v", err)
	}

	overMax := writeUploadFile(t, "over.txt", []byte("1234"))
	if err := validateCRMUploadFile(overMax, constraint); err == nil || !strings.Contains(err.Error(), "최대") {
		t.Fatalf("over max error: got %v", err)
	}

	dir := t.TempDir()
	if err := validateCRMUploadFile(dir, constraint); err == nil || !strings.Contains(err.Error(), "디렉터리") {
		t.Fatalf("directory error: got %v", err)
	}

	unsupported := writeUploadFile(t, "bad.bin", []byte("123"))
	if err := validateCRMUploadFile(unsupported, constraint); err == nil || !strings.Contains(err.Error(), "지원하지 않습니다") {
		t.Fatalf("unsupported extension error: got %v", err)
	}
}

func TestValidateCRMUploadFile_DocumentsImageUsesImageLimit(t *testing.T) {
	constraint := crmUploadConstraint{
		label:         "문서 첨부파일",
		maxBytes:      crmUploadMax20MB,
		maxBytesByExt: crmDocUploadMaxByExt,
		extensions:    crmDocUploadExt,
	}
	atMax := writeSizedUploadFile(t, "image.jpg", crmUploadMax10MB)
	if err := validateCRMUploadFile(atMax, constraint); err != nil {
		t.Fatalf("image at document image max should pass: %v", err)
	}

	overMax := writeSizedUploadFile(t, "large.jpg", crmUploadMax10MB+1)
	err := validateCRMUploadFile(overMax, constraint)
	if err == nil {
		t.Fatal("want document image size error")
	}
	if !strings.Contains(err.Error(), "10MB") {
		t.Fatalf("error should explain image-specific 10MB limit: %v", err)
	}
}

func TestValidateCRMUploadFile_AgentFileUsesTypeSpecificLimits(t *testing.T) {
	constraint := crmUploadConstraint{
		label:         "에이전트 파일",
		maxBytes:      crmUploadMax20MB,
		maxBytesByExt: crmAgentUploadMaxByExt,
		extensions:    crmAgentUploadExt,
	}
	pdfAtMax := writeSizedUploadFile(t, "source.pdf", crmUploadMax20MB)
	if err := validateCRMUploadFile(pdfAtMax, constraint); err != nil {
		t.Fatalf("PDF at agent PDF max should pass: %v", err)
	}

	textOverMax := writeSizedUploadFile(t, "source.txt", crmUploadMax1MB+1)
	err := validateCRMUploadFile(textOverMax, constraint)
	if err == nil {
		t.Fatal("want text file size error")
	}
	if !strings.Contains(err.Error(), "1MB") {
		t.Fatalf("error should explain text-specific 1MB limit: %v", err)
	}
}

func TestValidateCRMUploadFile_TemplateVersionAttachmentHasNoLocalTypeOrSizeLimit(t *testing.T) {
	constraint := crmUploadConstraint{
		label:         "첨부파일",
		maxBytes:      0,
		maxBytesByExt: nil,
		extensions:    nil,
	}
	filePath := writeSizedUploadFile(t, "archive.zip", crmUploadMax20MB+1)
	if err := validateCRMUploadFile(filePath, constraint); err != nil {
		t.Fatalf("unfiltered template attachment should pass local validation: %v", err)
	}
}
