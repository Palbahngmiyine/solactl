package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/solapi/solactl/pkg/client"
	"github.com/solapi/solactl/pkg/crm/spec"
)

const fakeSpec = `{
  "openapi": "3.0.0",
  "info": {"title": "crm", "version": "1"},
  "paths": {
    "/crm-core/v1/entities": {
      "get": {"summary": "list entities"}
    },
    "/crm-core/v1/records": {
      "get": {
        "summary": "list records",
        "parameters": [
          {"name": "entityId", "in": "query", "required": true},
          {"name": "limit", "in": "query"}
        ]
      },
      "post": {
        "summary": "create record",
        "requestBody": {"required": true}
      }
    },
    "/crm-core/v1/records/{id}": {
      "get": {
        "summary": "get record",
        "parameters": [
          {"name": "id", "in": "path", "required": true}
        ]
      },
      "delete": {
        "summary": "delete record",
        "parameters": [
          {"name": "id", "in": "path", "required": true}
        ]
      }
    }
  }
}`

func setupCRMTest(t *testing.T, handler http.HandlerFunc) (stdout, stderr *bytes.Buffer, _ func()) {
	t.Helper()
	resetFlags()
	t.Setenv("HOME", t.TempDir())
	t.Setenv(spec.CacheDirEnv, t.TempDir())
	t.Setenv("SOLACTL_API_KEY", "TESTKEY")
	t.Setenv("SOLACTL_API_SECRET", "TESTSECRET")

	specSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, fakeSpec)
	}))
	apiSrv := httptest.NewServer(handler)

	crmLoaderOverride = &spec.Loader{URL: specSrv.URL}
	c := &client.Client{
		HTTPClient:      apiSrv.Client(),
		APIKey:          "TESTKEY",
		APISecret:       "TESTSECRET",
		MaxRetries:      0,
		BaseDelay:       time.Millisecond,
		BaseURLOverride: apiSrv.URL,
	}
	clientOverride = c

	var outBuf, errBuf bytes.Buffer
	outWriter = &outBuf
	errWriter = &errBuf

	// Detach existing dynamic children so consecutive tests start clean.
	resetCRMRegistration()

	cleanup := func() {
		crmLoaderOverride = nil
		clientOverride = nil
		outWriter = nil
		errWriter = nil
		specSrv.Close()
		apiSrv.Close()
		resetFlags()
		resetCRMRegistration()
	}
	t.Cleanup(cleanup)

	RegisterDynamicCRM(context.Background())
	return &outBuf, &errBuf, cleanup
}

// resetCRMRegistration drops every dynamic resource subcommand so tests
// can re-register against a fresh spec without stale entries colliding.
func resetCRMRegistration() {
	if crmCmd == nil {
		return
	}
	for _, c := range crmCmd.Commands() {
		// Keep static children (`config`, `mcp` once added).
		if c.Use == "config" || strings.HasPrefix(c.Use, "config ") || c.Use == "mcp" {
			continue
		}
		crmCmd.RemoveCommand(c)
	}
}

func TestCRM_DynamicListRecords_TableFormat(t *testing.T) {
	stdout, _, _ := setupCRMTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if got := r.URL.Query().Get("entityId"); got != "ENxxx" {
			t.Errorf("entityId: got %q", got)
		}
		_, _ = io.WriteString(w, `[{"id":"R1","name":"홍길동"},{"id":"R2","name":"김철수"}]`)
	})

	rootCmd.SetArgs([]string{"crm", "records", "list", "--entityId", "ENxxx"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "R1") || !strings.Contains(out, "홍길동") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestCRM_DynamicListRecords_FormatJSON(t *testing.T) {
	stdout, _, _ := setupCRMTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[{"id":"R1"}]`)
	})

	rootCmd.SetArgs([]string{"crm", "records", "list", "--entityId", "X", "--format", "json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("err: %v", err)
	}
	var parsed []map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &parsed); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout.String())
	}
}

func TestCRM_DynamicListRecords_FormatCSV(t *testing.T) {
	stdout, _, _ := setupCRMTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[{"id":"R1","name":"a,b"}]`)
	})

	rootCmd.SetArgs([]string{"crm", "records", "list", "--entityId", "X", "--format", "csv"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(stdout.String(), `"a,b"`) {
		t.Errorf("CSV not escaped:\n%s", stdout.String())
	}
}

func TestCRM_DynamicGetRecord_PathParam(t *testing.T) {
	var sawPath string
	stdout, _, _ := setupCRMTest(t, func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		_, _ = io.WriteString(w, `{"id":"REC123","name":"홍길동"}`)
	})

	rootCmd.SetArgs([]string{"crm", "records", "get", "REC123", "--format", "json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(sawPath, "/crm-core/v1/records/REC123") {
		t.Errorf("path substitution failed: %s", sawPath)
	}
	if !strings.Contains(stdout.String(), "REC123") {
		t.Errorf("output missing payload: %s", stdout.String())
	}
}

func TestCRM_DynamicGetRecord_PathParamEncoded(t *testing.T) {
	var sawRequestURI string
	setupCRMTest(t, func(w http.ResponseWriter, r *http.Request) {
		// RequestURI preserves percent-encoding; URL.Path is already decoded.
		sawRequestURI = r.RequestURI
		_, _ = io.WriteString(w, `{}`)
	})

	rootCmd.SetArgs([]string{"crm", "records", "get", "abc/def", "--format", "json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("err: %v", err)
	}
	// `/` in arg must be percent-encoded to keep the path structure intact.
	if !strings.Contains(sawRequestURI, "abc%2Fdef") {
		t.Errorf("path arg not encoded: %s", sawRequestURI)
	}
}

func TestCRM_EncodePathArgMatchesURLPathEscape(t *testing.T) {
	tests := []string{
		"abc/def",
		"abc?def#ghi",
		"space value",
		"홍길동",
	}
	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			if got, want := encodePathArg(tc), url.PathEscape(tc); got != want {
				t.Fatalf("encodePathArg(%q) = %q, want %q", tc, got, want)
			}
		})
	}
}

func TestCRM_DynamicCreateRecord_DataFlag(t *testing.T) {
	var receivedBody map[string]any
	setupCRMTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedBody)
		_, _ = io.WriteString(w, `{"id":"R1"}`)
	})

	rootCmd.SetArgs([]string{"crm", "records", "create", "--data", `{"name":"홍길동"}`, "--format", "json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if got := receivedBody["name"]; got != "홍길동" {
		t.Errorf("body forwarding failed: %v", receivedBody)
	}
}

func TestCRM_DynamicCreateRecord_DataFileFlag(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/body.json"
	if err := writeFile(path, []byte(`{"name":"파일"}`)); err != nil {
		t.Fatal(err)
	}
	var receivedBody map[string]any
	setupCRMTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedBody)
		_, _ = io.WriteString(w, `{"ok":true}`)
	})

	rootCmd.SetArgs([]string{"crm", "records", "create", "--data-file", path})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if got := receivedBody["name"]; got != "파일" {
		t.Errorf("body file ignored: %v", receivedBody)
	}
}

func TestCRM_DynamicCreate_DataMissingErrors(t *testing.T) {
	setupCRMTest(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Errorf("server should not be called when body is required and missing")
		_, _ = io.WriteString(w, `{}`)
	})

	rootCmd.SetArgs([]string{"crm", "records", "create"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("want error when --data missing for required body")
	}
	if !strings.Contains(err.Error(), "--data") {
		t.Errorf("error should mention --data: %v", err)
	}
}

func TestCRM_DynamicCreate_DataInvalidJSON(t *testing.T) {
	setupCRMTest(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Errorf("server should not be called for malformed JSON")
		_, _ = io.WriteString(w, `{}`)
	})

	rootCmd.SetArgs([]string{"crm", "records", "create", "--data", `{not json}`})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("want JSON parse error")
	}
	if !strings.Contains(err.Error(), "JSON") {
		t.Errorf("error should mention JSON: %v", err)
	}
}

func TestCRM_DynamicDelete_NoBody(t *testing.T) {
	var sawMethod string
	setupCRMTest(t, func(w http.ResponseWriter, r *http.Request) {
		sawMethod = r.Method
		w.WriteHeader(204)
	})
	rootCmd.SetArgs([]string{"crm", "records", "delete", "REC1"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if sawMethod != http.MethodDelete {
		t.Errorf("method: got %s", sawMethod)
	}
}

func TestCRM_DynamicListRecords_RequiredQueryEnforced(t *testing.T) {
	setupCRMTest(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Errorf("server should not be called when required flag missing")
	})

	rootCmd.SetArgs([]string{"crm", "records", "list"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("want error when required --entityId missing")
	}
}

func TestCRM_DynamicListRecords_FormatInvalid(t *testing.T) {
	setupCRMTest(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[]`)
	})
	rootCmd.SetArgs([]string{"crm", "records", "list", "--entityId", "X", "--format", "yaml"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("want error for unknown format")
	}
}

func TestCRM_ConfigClearCache(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv(spec.CacheDirEnv, cacheDir)
	resetFlags()

	// Seed a file.
	if err := writeFile(cacheDir+"/openapi-spec-solapi.json", []byte(`{"data":{},"timestamp":0}`)); err != nil {
		t.Fatal(err)
	}

	var errBuf bytes.Buffer
	errWriter = &errBuf
	t.Cleanup(func() { errWriter = nil; resetFlags() })

	rootCmd.SetArgs([]string{"crm", "config", "clear-cache"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(errBuf.String(), "캐시 디렉토리를 비웠습니다") {
		t.Errorf("missing confirmation: %s", errBuf.String())
	}
}

func TestCRM_RegisterDynamicWithoutCredentials(t *testing.T) {
	resetFlags()
	t.Setenv("HOME", t.TempDir())
	t.Setenv(spec.CacheDirEnv, t.TempDir())
	t.Setenv("SOLACTL_API_KEY", "")
	t.Setenv("SOLACTL_API_SECRET", "")

	resetCRMRegistration()
	specSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, fakeSpec)
	}))
	t.Cleanup(specSrv.Close)
	crmLoaderOverride = &spec.Loader{URL: specSrv.URL}
	t.Cleanup(func() {
		crmLoaderOverride = nil
		resetCRMRegistration()
		resetFlags()
	})

	RegisterDynamicCRM(context.Background())

	var foundRecords bool
	for _, c := range crmCmd.Commands() {
		if c.Use == "records" {
			foundRecords = true
		}
	}
	if !foundRecords {
		t.Fatal("dynamic CRM commands should register before credentials are parsed")
	}
}

func writeFile(path string, b []byte) error {
	return os.WriteFile(path, b, 0o600)
}
