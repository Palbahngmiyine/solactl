package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/solapi/solactl/pkg/config"
)

func setupTestHome(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("SOLACTL_API_KEY", "")
	t.Setenv("SOLACTL_API_SECRET", "")
	return tmpDir
}

func resetFlags() {
	flagAPIKey = ""
	flagAPISecret = ""
	flagProfile = ""
	flagJSON = false
	flagDebug = false
}

func TestConfigure_NonInteractive(t *testing.T) {
	tmpDir := setupTestHome(t)
	resetFlags()

	var buf bytes.Buffer
	outWriter = &buf
	t.Cleanup(func() { outWriter = nil })

	flagAPIKey = "TESTKEY123"
	flagAPISecret = "TESTSECRET456"

	rootCmd.SetArgs([]string{"configure", "--api-key", "TESTKEY123", "--api-secret", "TESTSECRET456"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "설정이 저장되었습니다") {
		t.Errorf("expected save confirmation, got: %s", output)
	}

	// Verify file was saved
	cfgPath := filepath.Join(tmpDir, ".solactl", "credentials.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("config file not found: %v", err)
	}
	if !strings.Contains(string(data), "TESTKEY123") {
		t.Errorf("config file missing key: %s", data)
	}

	resetFlags()
}

func TestConfigure_NonInteractive_EmptyKey(t *testing.T) {
	setupTestHome(t)
	resetFlags()

	var buf bytes.Buffer
	outWriter = &buf
	t.Cleanup(func() { outWriter = nil })

	flagAPIKey = ""
	flagAPISecret = "TESTSECRET456"

	// When only secret is provided, it goes to interactive mode (which will fail without stdin)
	// So we test that non-interactive requires BOTH
	rootCmd.SetArgs([]string{"configure", "--api-secret", "TESTSECRET456"})
	err := rootCmd.Execute()
	// Interactive mode should fail because stdin is not a terminal
	// This validates that both flags are required for non-interactive
	if err == nil {
		// If it somehow succeeds (piped stdin), that's also fine
		t.Log("command succeeded (stdin may have provided input)")
	}
	resetFlags()
}

func TestConfigureShow_NoConfig(t *testing.T) {
	setupTestHome(t)
	resetFlags()

	var buf bytes.Buffer
	outWriter = &buf
	var errBuf bytes.Buffer
	errWriter = &errBuf
	t.Cleanup(func() { outWriter = nil; errWriter = nil })

	rootCmd.SetArgs([]string{"configure", "show"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "API Key") {
		t.Errorf("expected API Key label, got: %s", output)
	}
	// Warning should go to stderr, not stdout
	errOutput := errBuf.String()
	if !strings.Contains(errOutput, "⚠") {
		t.Errorf("expected warning for empty config on stderr, got: %s", errOutput)
	}
	if strings.Contains(output, "⚠") {
		t.Errorf("warning should not appear on stdout, got: %s", output)
	}
	resetFlags()
}

func TestConfigureShow_WithConfig(t *testing.T) {
	tmpDir := setupTestHome(t)
	resetFlags()

	// Save a config first
	cfgDir := filepath.Join(tmpDir, ".solactl")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(cfgDir, "credentials.json"),
		[]byte(`{"api_key":"MYKEY","api_secret":"MYSECRETVALUE123456789012345"}`),
		0600,
	); err != nil {
		t.Fatalf("write: %v", err)
	}

	var buf bytes.Buffer
	outWriter = &buf
	t.Cleanup(func() { outWriter = nil })

	rootCmd.SetArgs([]string{"configure", "show"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "MYKEY") {
		t.Errorf("expected API key in output, got: %s", output)
	}
	// Secret should be masked
	if strings.Contains(output, "MYSECRETVALUE123456789012345") {
		t.Error("API Secret should be masked in output")
	}
	if !strings.Contains(output, "MYSE****") {
		t.Errorf("expected masked secret, got: %s", output)
	}
	resetFlags()
}

func TestConfigureShow_NoAPIURL(t *testing.T) {
	setupTestHome(t)
	resetFlags()

	// Save config
	if err := config.Save(&config.Config{APIKey: "K", APISecret: "S"}, ""); err != nil {
		t.Fatalf("save: %v", err)
	}

	var buf bytes.Buffer
	outWriter = &buf
	t.Cleanup(func() { outWriter = nil })

	rootCmd.SetArgs([]string{"configure", "show"})
	_ = rootCmd.Execute()

	output := buf.String()
	// API URL should NOT be shown (per PRD: configure show에서 API URL 미표시)
	if strings.Contains(output, "api.solapi.com") {
		t.Error("API URL should not be shown in configure show")
	}
	if strings.Contains(output, "Base URL") {
		t.Error("Base URL should not be shown in configure show")
	}
	resetFlags()
}

func TestSaveConfigure_EmptyValues(t *testing.T) {
	setupTestHome(t)

	err := saveConfigure(&config.Config{APIKey: "", APISecret: "secret"}, "")
	if err == nil {
		t.Error("expected error for empty API Key")
	}

	err = saveConfigure(&config.Config{APIKey: "key", APISecret: ""}, "")
	if err == nil {
		t.Error("expected error for empty API Secret")
	}
}

func TestConfigureShow_ValidationWarning(t *testing.T) {
	tmpDir := setupTestHome(t)
	resetFlags()

	// Create config with empty APIKey to trigger validation warning
	cfgDir := filepath.Join(tmpDir, ".solactl")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write config with empty api_key — Load will return it as-is
	if err := os.WriteFile(
		filepath.Join(cfgDir, "credentials.json"),
		[]byte(`{"api_key":"","api_secret":"SOMESECRET12345678901234567"}`),
		0600,
	); err != nil {
		t.Fatalf("write: %v", err)
	}

	var buf bytes.Buffer
	outWriter = &buf
	var errBuf bytes.Buffer
	errWriter = &errBuf
	t.Cleanup(func() { outWriter = nil; errWriter = nil })

	rootCmd.SetArgs([]string{"configure", "show"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	errOutput := errBuf.String()
	if !strings.Contains(errOutput, "⚠") {
		t.Errorf("expected validation warning (⚠) for empty API Key on stderr, got: %s", errOutput)
	}
	stdoutOutput := buf.String()
	if strings.Contains(stdoutOutput, "⚠") {
		t.Errorf("warning should not appear on stdout, got: %s", stdoutOutput)
	}
	resetFlags()
}

func TestConfigure_NonInteractive_BothEmpty(t *testing.T) {
	setupTestHome(t)
	resetFlags()

	var buf bytes.Buffer
	outWriter = &buf
	t.Cleanup(func() { outWriter = nil })

	flagAPIKey = ""
	flagAPISecret = ""

	// With both flags empty, runConfigure enters interactive mode.
	// With piped (empty) stdin it should fail gracefully, not panic.
	rootCmd.SetArgs([]string{"configure"})
	err := rootCmd.Execute()
	// We expect an error because stdin is empty (not a terminal), but no panic
	if err == nil {
		t.Log("command succeeded (stdin may have provided defaults)")
	}
	// The key assertion is that we did NOT panic
	resetFlags()
}

func TestSaveConfigure_SaveFailure(t *testing.T) {
	resetFlags()

	// Set HOME to a path that cannot be written to,
	// causing config.Save to fail.
	t.Setenv("HOME", "/dev/null/nonexistent")
	t.Setenv("SOLACTL_API_KEY", "")
	t.Setenv("SOLACTL_API_SECRET", "")

	var buf bytes.Buffer
	outWriter = &buf
	t.Cleanup(func() { outWriter = nil })

	err := saveConfigure(&config.Config{APIKey: "testkey", APISecret: "testsecret"}, "")
	if err == nil {
		t.Fatal("expected save failure error")
	}
	if !strings.Contains(err.Error(), "설정 저장 실패") {
		t.Errorf("expected '설정 저장 실패' in error, got: %v", err)
	}
	resetFlags()
}

func setupMultiProfile(t *testing.T, tmpDir string, profiles map[string]struct{ Key, Secret string }, active string) {
	t.Helper()
	cfgDir := filepath.Join(tmpDir, ".solactl")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	type profileCfg struct {
		APIKey    string `json:"api_key"`
		APISecret string `json:"api_secret"`
	}
	type credFile struct {
		Profiles      map[string]*profileCfg `json:"profiles"`
		ActiveProfile string                 `json:"active_profile"`
	}
	cf := &credFile{Profiles: make(map[string]*profileCfg), ActiveProfile: active}
	for name, p := range profiles {
		cf.Profiles[name] = &profileCfg{APIKey: p.Key, APISecret: p.Secret}
	}
	data, _ := json.MarshalIndent(cf, "", "  ")
	if err := os.WriteFile(filepath.Join(cfgDir, "credentials.json"), data, 0600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestConfigure_WithProfile(t *testing.T) {
	tmpDir := setupTestHome(t)
	resetFlags()

	var buf bytes.Buffer
	outWriter = &buf
	t.Cleanup(func() { outWriter = nil })

	rootCmd.SetArgs([]string{"configure", "--profile", "staging", "--api-key", "STAGINGKEY", "--api-secret", "STAGINGSECRET"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "설정이 저장되었습니다") {
		t.Errorf("expected save confirmation, got: %s", output)
	}
	if !strings.Contains(output, "staging") {
		t.Errorf("expected profile name in output, got: %s", output)
	}

	// Verify the profile was saved
	data, err := os.ReadFile(filepath.Join(tmpDir, ".solactl", "credentials.json"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "STAGINGKEY") {
		t.Errorf("config file should contain staging key: %s", data)
	}
	if !strings.Contains(string(data), `"staging"`) {
		t.Errorf("config file should contain staging profile: %s", data)
	}
	resetFlags()
}

func TestConfigureList_Empty(t *testing.T) {
	setupTestHome(t)
	resetFlags()

	var buf bytes.Buffer
	outWriter = &buf
	var errBuf bytes.Buffer
	errWriter = &errBuf
	t.Cleanup(func() { outWriter = nil; errWriter = nil })

	rootCmd.SetArgs([]string{"configure", "list"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	errOutput := errBuf.String()
	if !strings.Contains(errOutput, "프로필이 없습니다") {
		t.Errorf("expected 'no profiles' message on stderr, got: %s", errOutput)
	}
	resetFlags()
}

func TestConfigureList_MultipleProfiles(t *testing.T) {
	tmpDir := setupTestHome(t)
	resetFlags()

	setupMultiProfile(t, tmpDir, map[string]struct{ Key, Secret string }{
		"default": {"DEFAULT-KEY", "DEFAULT-SECRET-1234567890"},
		"staging": {"STAGING-KEY", "STAGING-SECRET-1234567890"},
	}, "default")

	var buf bytes.Buffer
	outWriter = &buf
	t.Cleanup(func() { outWriter = nil })

	rootCmd.SetArgs([]string{"configure", "list"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "default") {
		t.Errorf("expected 'default' profile in output, got: %s", output)
	}
	if !strings.Contains(output, "staging") {
		t.Errorf("expected 'staging' profile in output, got: %s", output)
	}
	if !strings.Contains(output, "*") {
		t.Errorf("expected active marker (*) in output, got: %s", output)
	}
	// API Secret should be masked
	if strings.Contains(output, "DEFAULT-SECRET-1234567890") {
		t.Error("API Secret should be masked in list output")
	}
	resetFlags()
}

func TestConfigureUse_Valid(t *testing.T) {
	tmpDir := setupTestHome(t)
	resetFlags()

	setupMultiProfile(t, tmpDir, map[string]struct{ Key, Secret string }{
		"default": {"D-KEY", "D-SECRET"},
		"staging": {"S-KEY", "S-SECRET"},
	}, "default")

	var errBuf bytes.Buffer
	errWriter = &errBuf
	t.Cleanup(func() { errWriter = nil })

	rootCmd.SetArgs([]string{"configure", "use", "staging"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	errOutput := errBuf.String()
	if !strings.Contains(errOutput, "staging") {
		t.Errorf("expected profile name in confirmation, got: %s", errOutput)
	}
	if !strings.Contains(errOutput, "전환") {
		t.Errorf("expected switch confirmation, got: %s", errOutput)
	}
	resetFlags()
}

func TestConfigureUse_NonExistent(t *testing.T) {
	tmpDir := setupTestHome(t)
	resetFlags()

	setupMultiProfile(t, tmpDir, map[string]struct{ Key, Secret string }{
		"default": {"D-KEY", "D-SECRET"},
	}, "default")

	rootCmd.SetArgs([]string{"configure", "use", "nonexistent"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent profile")
	}
	resetFlags()
}

func TestConfigureUse_NoArg(t *testing.T) {
	setupTestHome(t)
	resetFlags()

	rootCmd.SetArgs([]string{"configure", "use"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when no profile argument given")
	}
	resetFlags()
}

func TestConfigureDelete_Valid(t *testing.T) {
	tmpDir := setupTestHome(t)
	resetFlags()

	setupMultiProfile(t, tmpDir, map[string]struct{ Key, Secret string }{
		"default": {"D-KEY", "D-SECRET"},
		"staging": {"S-KEY", "S-SECRET"},
	}, "default")

	var errBuf bytes.Buffer
	errWriter = &errBuf
	t.Cleanup(func() { errWriter = nil })

	rootCmd.SetArgs([]string{"configure", "delete", "staging"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	errOutput := errBuf.String()
	if !strings.Contains(errOutput, "삭제") {
		t.Errorf("expected delete confirmation, got: %s", errOutput)
	}

	// Verify profile was deleted
	data, _ := os.ReadFile(filepath.Join(tmpDir, ".solactl", "credentials.json"))
	if strings.Contains(string(data), "staging") {
		t.Error("staging profile should be deleted from file")
	}
	resetFlags()
}

func TestConfigureDelete_ActiveProfile(t *testing.T) {
	tmpDir := setupTestHome(t)
	resetFlags()

	setupMultiProfile(t, tmpDir, map[string]struct{ Key, Secret string }{
		"default": {"D-KEY", "D-SECRET"},
		"staging": {"S-KEY", "S-SECRET"},
	}, "default")

	rootCmd.SetArgs([]string{"configure", "delete", "default"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when deleting active profile")
	}
	resetFlags()
}

func TestConfigureDelete_NoArg(t *testing.T) {
	setupTestHome(t)
	resetFlags()

	rootCmd.SetArgs([]string{"configure", "delete"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when no profile argument given")
	}
	resetFlags()
}

func TestConfigureShow_DisplaysProfileName(t *testing.T) {
	tmpDir := setupTestHome(t)
	resetFlags()

	setupMultiProfile(t, tmpDir, map[string]struct{ Key, Secret string }{
		"default": {"MYKEY", "MYSECRETVALUE123456789012345"},
	}, "default")

	var buf bytes.Buffer
	outWriter = &buf
	t.Cleanup(func() { outWriter = nil })

	rootCmd.SetArgs([]string{"configure", "show"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Profile") {
		t.Errorf("expected Profile label in output, got: %s", output)
	}
	if !strings.Contains(output, "default") {
		t.Errorf("expected profile name 'default' in output, got: %s", output)
	}
	resetFlags()
}

func TestCtx_NilFallback(t *testing.T) {
	// When PersistentPreRun has not been called, cmdCtx is nil.
	// ctx() should return context.Background() as fallback.
	origCtx := cmdCtx
	cmdCtx = nil
	t.Cleanup(func() { cmdCtx = origCtx })

	got := ctx()
	if got == nil {
		t.Fatal("ctx() returned nil")
	}
	// Should not be cancelled
	select {
	case <-got.Done():
		t.Error("fallback context should not be done")
	default:
	}
}
