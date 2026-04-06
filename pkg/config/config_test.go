package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid",
			cfg:     Config{APIKey: "test-api-key", APISecret: "test-api-secret"},
			wantErr: false,
		},
		{
			name:    "missing api key",
			cfg:     Config{APISecret: "test-api-secret"},
			wantErr: true,
			errMsg:  "API Key가 설정되지 않았습니다",
		},
		{
			name:    "missing api secret",
			cfg:     Config{APIKey: "test-api-key"},
			wantErr: true,
			errMsg:  "API Secret이 설정되지 않았습니다",
		},
		{
			name:    "both empty",
			cfg:     Config{},
			wantErr: true,
			errMsg:  "API Key가 설정되지 않았습니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errMsg != "" && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Validate() error = %q, want to contain %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestConfig_Validate_ContainsSolactlConfigure(t *testing.T) {
	cfg := Config{}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty config")
	}
	if !strings.Contains(err.Error(), "solactl configure") {
		t.Errorf("error should mention 'solactl configure', got: %v", err)
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SOLACTL_API_KEY", "")
	t.Setenv("SOLACTL_API_SECRET", "")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "" {
		t.Errorf("APIKey: got %q, want empty", cfg.APIKey)
	}
	if cfg.APISecret != "" {
		t.Errorf("APISecret: got %q, want empty", cfg.APISecret)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SOLACTL_API_KEY", "env-key-123")
	t.Setenv("SOLACTL_API_SECRET", "env-secret-456")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "env-key-123" {
		t.Errorf("APIKey: got %q", cfg.APIKey)
	}
	if cfg.APISecret != "env-secret-456" {
		t.Errorf("APISecret: got %q", cfg.APISecret)
	}
}

func TestLoad_FlagOverrideBeatsEnv(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SOLACTL_API_KEY", "env-key-123")
	t.Setenv("SOLACTL_API_SECRET", "")

	overrides := &Config{
		APIKey: "flag-key-override",
	}

	cfg, err := Load(overrides)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "flag-key-override" {
		t.Errorf("APIKey should be flag value: got %q", cfg.APIKey)
	}
}

func TestLoad_FileConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("SOLACTL_API_KEY", "")
	t.Setenv("SOLACTL_API_SECRET", "")

	cfgDir := filepath.Join(tmpDir, configDir)
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	cfgPath := filepath.Join(cfgDir, configFile)
	if err := os.WriteFile(cfgPath, []byte(`{"api_key":"file-key","api_secret":"file-secret"}`), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "file-key" {
		t.Errorf("APIKey: got %q", cfg.APIKey)
	}
	if cfg.APISecret != "file-secret" {
		t.Errorf("APISecret: got %q", cfg.APISecret)
	}
}

func TestLoad_BrokenJSONFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("SOLACTL_API_KEY", "")
	t.Setenv("SOLACTL_API_SECRET", "")

	cfgDir := filepath.Join(tmpDir, configDir)
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	cfgPath := filepath.Join(cfgDir, configFile)
	if err := os.WriteFile(cfgPath, []byte(`{broken json`), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Should not fail -- broken file is ignored, defaults used
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "" {
		t.Errorf("APIKey: got %q, want empty (defaults)", cfg.APIKey)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("SOLACTL_API_KEY", "")
	t.Setenv("SOLACTL_API_SECRET", "")

	original := &Config{
		APIKey:    "save-key-123",
		APISecret: "save-secret-456",
	}

	if err := Save(original); err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := Load(nil)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if loaded.APIKey != original.APIKey {
		t.Errorf("APIKey: got %q, want %q", loaded.APIKey, original.APIKey)
	}
	if loaded.APISecret != original.APISecret {
		t.Errorf("APISecret: got %q, want %q", loaded.APISecret, original.APISecret)
	}
}

func TestSave_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg := &Config{
		APIKey:    "idem-key",
		APISecret: "idem-secret",
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("first save: %v", err)
	}
	first, err := os.ReadFile(filepath.Join(tmpDir, configDir, configFile))
	if err != nil {
		t.Fatalf("read first: %v", err)
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("second save: %v", err)
	}
	second, err := os.ReadFile(filepath.Join(tmpDir, configDir, configFile))
	if err != nil {
		t.Fatalf("read second: %v", err)
	}

	if string(first) != string(second) {
		t.Error("idempotency violation: two saves of the same config produced different files")
	}
}

func TestSave_FailurePreservesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Save initial config successfully
	initial := &Config{
		APIKey:    "initial-key",
		APISecret: "initial-secret",
	}
	if err := Save(initial); err != nil {
		t.Fatalf("initial save: %v", err)
	}

	// Read the saved file content
	cfgPath := filepath.Join(tmpDir, configDir, configFile)
	originalData, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read original: %v", err)
	}

	// Now break HOME so next Save fails
	t.Setenv("HOME", "/nonexistent/path/that/does/not/exist")

	updated := &Config{
		APIKey:    "updated-key",
		APISecret: "updated-secret",
	}
	_ = Save(updated) // expected to fail

	// Restore HOME and verify original file is unchanged
	t.Setenv("HOME", tmpDir)
	afterData, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read after failed save: %v", err)
	}

	if string(originalData) != string(afterData) {
		t.Error("failed save corrupted existing config file")
	}
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"01234567890123456789012345678901", "0123****************************"},
		{"ab", "****"},
		{"", "****"},
		{"abcd", "****"},
		{"abcde", "abcd****************************"},
	}

	for _, tt := range tests {
		got := MaskSecret(tt.input)
		if got != tt.want {
			t.Errorf("MaskSecret(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestConfigFilePath_ReturnsPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	path, err := ConfigFilePath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	suffix := filepath.Join(".solactl", "credentials.json")
	if !strings.HasSuffix(path, suffix) {
		t.Errorf("ConfigFilePath() = %q, want suffix %q", path, suffix)
	}
}

func TestSave_MkdirAllFailure(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create a regular file at $HOME/.solactl to block directory creation
	if err := os.WriteFile(filepath.Join(tmpDir, ".solactl"), []byte("block"), 0400); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	err := Save(&Config{APIKey: "k", APISecret: "s"})
	if err == nil {
		t.Fatal("expected error when directory creation is blocked, got nil")
	}
	if !strings.Contains(err.Error(), "디렉토리 생성 실패") {
		t.Errorf("error should contain '디렉토리 생성 실패', got: %v", err)
	}
}

func TestLoadFromFile_NoFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	_, err := LoadFromFile()
	if err == nil {
		t.Fatal("expected error when config file doesn't exist")
	}
}

func TestLoadFromFile_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfgDir := filepath.Join(tmpDir, configDir)
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	cfgPath := filepath.Join(cfgDir, configFile)
	if err := os.WriteFile(cfgPath, []byte(`{"api_key":"k1","api_secret":"s1"}`), 0600); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	cfg, err := LoadFromFile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "k1" {
		t.Errorf("APIKey: got %q, want %q", cfg.APIKey, "k1")
	}
	if cfg.APISecret != "s1" {
		t.Errorf("APISecret: got %q, want %q", cfg.APISecret, "s1")
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Write config file
	cfgDir := filepath.Join(tmpDir, configDir)
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, configFile), []byte(`{"api_key":"file-key","api_secret":"file-secret"}`), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Env should override file
	t.Setenv("SOLACTL_API_KEY", "env-key")
	t.Setenv("SOLACTL_API_SECRET", "")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if cfg.APIKey != "env-key" {
		t.Errorf("APIKey: got %q, want %q (env should override file)", cfg.APIKey, "env-key")
	}
	if cfg.APISecret != "file-secret" {
		t.Errorf("APISecret: got %q, want %q (file value should remain)", cfg.APISecret, "file-secret")
	}
}

func FuzzMaskSecret(f *testing.F) {
	f.Add("")
	f.Add("ab")
	f.Add("abcde")
	f.Add("01234567890123456789012345678901")
	f.Fuzz(func(t *testing.T, s string) {
		// Must not panic
		result := MaskSecret(s)
		if result == "" {
			t.Error("MaskSecret should never return empty string")
		}
	})
}

func TestConcurrent_SaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("SOLACTL_API_KEY", "")
	t.Setenv("SOLACTL_API_SECRET", "")

	// Seed an initial config so Load always finds a valid file
	initial := &Config{APIKey: "init-key", APISecret: "init-secret"}
	if err := Save(initial); err != nil {
		t.Fatalf("initial save: %v", err)
	}

	// The primary goal of this test is to detect data races under -race.
	// File I/O via os.WriteFile is not atomic, so concurrent readers may
	// see partial writes. We only require that no goroutine panics and
	// the race detector stays clean.
	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	panicCh := make(chan string, goroutines)

	for i := 0; i < goroutines; i++ {
		if i%2 == 0 {
			go func(n int) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						panicCh <- fmt.Sprintf("save goroutine %d panicked: %v", n, r)
					}
				}()
				cfg := &Config{
					APIKey:    fmt.Sprintf("concurrent-key-%d", n),
					APISecret: fmt.Sprintf("concurrent-secret-%d", n),
				}
				_ = Save(cfg)
			}(i)
		} else {
			go func(n int) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						panicCh <- fmt.Sprintf("load goroutine %d panicked: %v", n, r)
					}
				}()
				// Load may fail or return partial data due to concurrent writes;
				// the key assertion is no panic and no data race.
				_, _ = Load(nil)
			}(i)
		}
	}

	wg.Wait()
	close(panicCh)

	for msg := range panicCh {
		t.Errorf("concurrent panic: %s", msg)
	}

	// After all goroutines finish, a final Load should return valid data
	finalCfg, err := Load(nil)
	if err != nil {
		t.Fatalf("final Load after concurrent access: %v", err)
	}
	if finalCfg.APIKey == "" {
		t.Error("final config has empty APIKey after concurrent Save/Load")
	}
	if finalCfg.APISecret == "" {
		t.Error("final config has empty APISecret after concurrent Save/Load")
	}
}

func TestMaskSecret_Boundary4Chars(t *testing.T) {
	got := MaskSecret("abcd")
	if got != "****" {
		t.Errorf("MaskSecret(%q) = %q, want %q", "abcd", got, "****")
	}
}

func TestMaskSecret_Boundary5Chars(t *testing.T) {
	got := MaskSecret("abcde")
	want := "abcd****************************"
	if got != want {
		t.Errorf("MaskSecret(%q) = %q, want %q", "abcde", got, want)
	}
}

func TestLoad_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("SOLACTL_API_KEY", "")
	t.Setenv("SOLACTL_API_SECRET", "")

	// Save a config so Load has something to read
	if err := Save(&Config{APIKey: "idem-key", APISecret: "idem-secret"}); err != nil {
		t.Fatalf("save: %v", err)
	}

	cfg1, err1 := Load(nil)
	if err1 != nil {
		t.Fatalf("first Load: %v", err1)
	}
	cfg2, err2 := Load(nil)
	if err2 != nil {
		t.Fatalf("second Load: %v", err2)
	}

	if cfg1.APIKey != cfg2.APIKey {
		t.Errorf("APIKey mismatch: %q vs %q", cfg1.APIKey, cfg2.APIKey)
	}
	if cfg1.APISecret != cfg2.APISecret {
		t.Errorf("APISecret mismatch: %q vs %q", cfg1.APISecret, cfg2.APISecret)
	}
}

func FuzzConfigJSON(f *testing.F) {
	f.Add([]byte(`{"api_key":"k","api_secret":"s"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(``))
	f.Add([]byte(`{"api_key":123}`))
	f.Add([]byte{0xff, 0xfe, 0xfd})

	f.Fuzz(func(t *testing.T, data []byte) {
		var cfg Config
		// Must not panic regardless of input
		_ = json.Unmarshal(data, &cfg)
	})
}

func TestSave_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg := &Config{APIKey: "perm-key", APISecret: "perm-secret"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Check directory permissions
	dirPath := filepath.Join(tmpDir, configDir)
	dirInfo, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	dirPerm := dirInfo.Mode().Perm()
	if dirPerm != 0700 {
		t.Errorf("directory permissions: got %o, want 0700", dirPerm)
	}

	// Check file permissions
	filePath := filepath.Join(dirPath, configFile)
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	filePerm := fileInfo.Mode().Perm()
	if filePerm != 0600 {
		t.Errorf("file permissions: got %o, want 0600", filePerm)
	}
}

func TestLoad_VeryLongValues(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("SOLACTL_API_KEY", "")
	t.Setenv("SOLACTL_API_SECRET", "")

	longKey := strings.Repeat("K", 10000)
	longSecret := strings.Repeat("S", 10000)

	original := &Config{APIKey: longKey, APISecret: longSecret}
	if err := Save(original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.APIKey != longKey {
		t.Errorf("APIKey length: got %d, want %d", len(loaded.APIKey), len(longKey))
	}
	if loaded.APISecret != longSecret {
		t.Errorf("APISecret length: got %d, want %d", len(loaded.APISecret), len(longSecret))
	}
}

func TestConfigDirPath_HomeError(t *testing.T) {
	// Set HOME to empty string to force os.UserHomeDir to fail
	t.Setenv("HOME", "")

	_, err := configDirPath()
	if err == nil {
		t.Fatal("expected error when HOME is empty, got nil")
	}
	if !strings.Contains(err.Error(), "홈 디렉토리를 찾을 수 없습니다") {
		t.Errorf("error should mention home directory lookup failure, got: %v", err)
	}
}
