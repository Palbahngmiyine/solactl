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

	cfg, err := Load(&LoadOptions{Overrides: overrides})
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

	if err := Save(original, ""); err != nil {
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

	if err := Save(cfg, ""); err != nil {
		t.Fatalf("first save: %v", err)
	}
	first, err := os.ReadFile(filepath.Join(tmpDir, configDir, configFile))
	if err != nil {
		t.Fatalf("read first: %v", err)
	}

	if err := Save(cfg, ""); err != nil {
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
	if err := Save(initial, ""); err != nil {
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
	_ = Save(updated, "") // expected to fail

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

	err := Save(&Config{APIKey: "k", APISecret: "s"}, "")
	if err == nil {
		t.Fatal("expected error when directory creation is blocked, got nil")
	}
	// Save now propagates load errors (not just mkdir errors)
	if !strings.Contains(err.Error(), "설정 파일 읽기 실패") && !strings.Contains(err.Error(), "디렉토리 생성 실패") {
		t.Errorf("error should indicate file/directory failure, got: %v", err)
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
	if err := Save(initial, ""); err != nil {
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
				_ = Save(cfg, "")
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

	// After all goroutines finish, verify the file is parseable.
	// With read-modify-write Save, concurrent file I/O may cause data
	// loss, but the file must still be valid JSON after all goroutines
	// complete. We only require a parseable file (or valid empty state).
	finalCfg, _ := Load(nil)
	if finalCfg == nil {
		t.Error("final Load returned nil Config after concurrent Save/Load")
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
	if err := Save(&Config{APIKey: "idem-key", APISecret: "idem-secret"}, ""); err != nil {
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
	if err := Save(cfg, ""); err != nil {
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
	if err := Save(original, ""); err != nil {
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

// --- Multi-profile tests ---

func setupMultiProfileFile(t *testing.T, tmpDir string, profiles map[string]*Config, active string) {
	t.Helper()
	cfgDir := filepath.Join(tmpDir, configDir)
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cf := &CredentialsFile{Profiles: profiles, ActiveProfile: active}
	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, configFile), data, 0600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestLoad_WithProfile(t *testing.T) {
	tests := []struct {
		name        string
		profiles    map[string]*Config
		active      string
		loadProfile string
		wantKey     string
		wantErr     bool
	}{
		{
			name: "load named profile",
			profiles: map[string]*Config{
				"default":  {APIKey: "default-key", APISecret: "default-secret"},
				"staging":  {APIKey: "staging-key", APISecret: "staging-secret"},
			},
			active:      "default",
			loadProfile: "staging",
			wantKey:     "staging-key",
		},
		{
			name: "load active profile when no profile specified",
			profiles: map[string]*Config{
				"default":  {APIKey: "default-key", APISecret: "default-secret"},
				"staging":  {APIKey: "staging-key", APISecret: "staging-secret"},
			},
			active:      "staging",
			loadProfile: "",
			wantKey:     "staging-key",
		},
		{
			name: "missing profile returns empty config (env/flags take precedence)",
			profiles: map[string]*Config{
				"default": {APIKey: "default-key", APISecret: "default-secret"},
			},
			active:      "default",
			loadProfile: "nonexistent",
			wantKey:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Setenv("HOME", tmpDir)
			t.Setenv("SOLACTL_API_KEY", "")
			t.Setenv("SOLACTL_API_SECRET", "")
			setupMultiProfileFile(t, tmpDir, tt.profiles, tt.active)

			cfg, err := Load(&LoadOptions{ProfileName: tt.loadProfile})
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.APIKey != tt.wantKey {
				t.Errorf("APIKey: got %q, want %q", cfg.APIKey, tt.wantKey)
			}
		})
	}
}

func TestLoad_ProfileOverriddenByEnv(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("SOLACTL_API_KEY", "env-key")
	t.Setenv("SOLACTL_API_SECRET", "")

	setupMultiProfileFile(t, tmpDir, map[string]*Config{
		"default": {APIKey: "file-key", APISecret: "file-secret"},
	}, "default")

	cfg, err := Load(&LoadOptions{ProfileName: "default"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "env-key" {
		t.Errorf("APIKey: got %q, want %q (env should override profile)", cfg.APIKey, "env-key")
	}
	if cfg.APISecret != "file-secret" {
		t.Errorf("APISecret: got %q, want %q (file value should remain)", cfg.APISecret, "file-secret")
	}
}

func TestLoad_BackwardCompatibility_FlatFormat(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("SOLACTL_API_KEY", "")
	t.Setenv("SOLACTL_API_SECRET", "")

	// Write old flat format
	cfgDir := filepath.Join(tmpDir, configDir)
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(cfgDir, configFile),
		[]byte(`{"api_key":"old-key","api_secret":"old-secret"}`),
		0600,
	); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "old-key" {
		t.Errorf("APIKey: got %q, want %q", cfg.APIKey, "old-key")
	}
	if cfg.APISecret != "old-secret" {
		t.Errorf("APISecret: got %q, want %q", cfg.APISecret, "old-secret")
	}
}

func TestSave_ToProfile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	setupMultiProfileFile(t, tmpDir, map[string]*Config{
		"default": {APIKey: "default-key", APISecret: "default-secret"},
	}, "default")

	// Save to a different profile
	if err := Save(&Config{APIKey: "staging-key", APISecret: "staging-secret"}, "staging"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify both profiles exist
	cf, err := LoadCredentialsFile()
	if err != nil {
		t.Fatalf("LoadCredentialsFile: %v", err)
	}
	if len(cf.Profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(cf.Profiles))
	}
	if cf.Profiles["default"].APIKey != "default-key" {
		t.Errorf("default profile should be preserved, got APIKey=%q", cf.Profiles["default"].APIKey)
	}
	if cf.Profiles["staging"].APIKey != "staging-key" {
		t.Errorf("staging profile APIKey: got %q", cf.Profiles["staging"].APIKey)
	}
	// Active profile should remain default (first profile)
	if cf.ActiveProfile != "default" {
		t.Errorf("ActiveProfile: got %q, want %q", cf.ActiveProfile, "default")
	}
}

func TestSave_EmptyProfileName(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	if err := Save(&Config{APIKey: "k", APISecret: "s"}, ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cf, err := LoadCredentialsFile()
	if err != nil {
		t.Fatalf("LoadCredentialsFile: %v", err)
	}
	if _, ok := cf.Profiles[DefaultProfile]; !ok {
		t.Error("empty profile name should default to 'default'")
	}
}

func TestSave_PreservesOtherProfiles(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	setupMultiProfileFile(t, tmpDir, map[string]*Config{
		"default": {APIKey: "d-key", APISecret: "d-secret"},
		"prod":    {APIKey: "p-key", APISecret: "p-secret"},
	}, "default")

	// Update only default profile
	if err := Save(&Config{APIKey: "d-key-v2", APISecret: "d-secret-v2"}, "default"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cf, err := LoadCredentialsFile()
	if err != nil {
		t.Fatalf("LoadCredentialsFile: %v", err)
	}
	if cf.Profiles["prod"].APIKey != "p-key" {
		t.Errorf("prod profile should be preserved, got APIKey=%q", cf.Profiles["prod"].APIKey)
	}
	if cf.Profiles["default"].APIKey != "d-key-v2" {
		t.Errorf("default profile should be updated, got APIKey=%q", cf.Profiles["default"].APIKey)
	}
}

func TestSetActiveProfile_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	setupMultiProfileFile(t, tmpDir, map[string]*Config{
		"default": {APIKey: "d-key", APISecret: "d-secret"},
		"staging": {APIKey: "s-key", APISecret: "s-secret"},
	}, "default")

	if err := SetActiveProfile("staging"); err != nil {
		t.Fatalf("SetActiveProfile: %v", err)
	}

	name, err := ActiveProfileName()
	if err != nil {
		t.Fatalf("ActiveProfileName: %v", err)
	}
	if name != "staging" {
		t.Errorf("ActiveProfile: got %q, want %q", name, "staging")
	}
}

func TestSetActiveProfile_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	setupMultiProfileFile(t, tmpDir, map[string]*Config{
		"default": {APIKey: "k", APISecret: "s"},
	}, "default")

	err := SetActiveProfile("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent profile")
	}
	if !strings.Contains(err.Error(), "찾을 수 없습니다") {
		t.Errorf("error should mention profile not found, got: %v", err)
	}
}

func TestSetActiveProfile_IOFailure(t *testing.T) {
	t.Setenv("HOME", "/dev/null/nonexistent")

	err := SetActiveProfile("any")
	if err == nil {
		t.Fatal("expected error when credentials file cannot be read")
	}
}

func TestDeleteProfile_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	setupMultiProfileFile(t, tmpDir, map[string]*Config{
		"default": {APIKey: "d-key", APISecret: "d-secret"},
		"staging": {APIKey: "s-key", APISecret: "s-secret"},
	}, "default")

	if err := DeleteProfile("staging"); err != nil {
		t.Fatalf("DeleteProfile: %v", err)
	}

	cf, err := LoadCredentialsFile()
	if err != nil {
		t.Fatalf("LoadCredentialsFile: %v", err)
	}
	if _, ok := cf.Profiles["staging"]; ok {
		t.Error("staging profile should be deleted")
	}
	if len(cf.Profiles) != 1 {
		t.Errorf("expected 1 profile remaining, got %d", len(cf.Profiles))
	}
}

func TestDeleteProfile_RejectsActiveProfile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	setupMultiProfileFile(t, tmpDir, map[string]*Config{
		"default": {APIKey: "d-key", APISecret: "d-secret"},
		"staging": {APIKey: "s-key", APISecret: "s-secret"},
	}, "default")

	err := DeleteProfile("default")
	if err == nil {
		t.Fatal("expected error when deleting active profile")
	}
	if !strings.Contains(err.Error(), "활성 프로필") {
		t.Errorf("error should mention active profile, got: %v", err)
	}
}

func TestDeleteProfile_RejectsLastProfile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// With inferActiveProfile, the sole profile is always inferred as active,
	// so the "active profile" guard fires before the "last profile" guard.
	setupMultiProfileFile(t, tmpDir, map[string]*Config{
		"only": {APIKey: "k", APISecret: "s"},
	}, "other")

	err := DeleteProfile("only")
	if err == nil {
		t.Fatal("expected error when deleting last profile")
	}
	// Either "활성 프로필" or "마지막 프로필" error is acceptable
	if !strings.Contains(err.Error(), "활성 프로필") && !strings.Contains(err.Error(), "마지막 프로필") {
		t.Errorf("error should reject deletion, got: %v", err)
	}
}

func TestDeleteProfile_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	setupMultiProfileFile(t, tmpDir, map[string]*Config{
		"default": {APIKey: "k", APISecret: "s"},
	}, "default")

	err := DeleteProfile("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent profile")
	}
	if !strings.Contains(err.Error(), "찾을 수 없습니다") {
		t.Errorf("error should mention profile not found, got: %v", err)
	}
}

func TestListProfiles_Empty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	profiles, err := ListProfiles()
	if err == nil && len(profiles) > 0 {
		t.Error("expected no profiles when credentials file doesn't exist")
	}
}

func TestListProfiles_Multiple(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	setupMultiProfileFile(t, tmpDir, map[string]*Config{
		"beta":    {APIKey: "beta-key", APISecret: "beta-secret"},
		"default": {APIKey: "d-key", APISecret: "d-secret"},
		"alpha":   {APIKey: "alpha-key", APISecret: "alpha-secret"},
	}, "default")

	profiles, err := ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	if len(profiles) != 3 {
		t.Fatalf("expected 3 profiles, got %d", len(profiles))
	}

	// Should be sorted by name
	if profiles[0].Name != "alpha" {
		t.Errorf("first profile: got %q, want %q", profiles[0].Name, "alpha")
	}
	if profiles[1].Name != "beta" {
		t.Errorf("second profile: got %q, want %q", profiles[1].Name, "beta")
	}
	if profiles[2].Name != "default" {
		t.Errorf("third profile: got %q, want %q", profiles[2].Name, "default")
	}

	// Active profile check
	for _, p := range profiles {
		if p.Name == "default" && !p.Active {
			t.Error("default profile should be marked as active")
		}
		if p.Name != "default" && p.Active {
			t.Errorf("profile %q should not be marked as active", p.Name)
		}
	}
}

func TestMigration_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("SOLACTL_API_KEY", "")
	t.Setenv("SOLACTL_API_SECRET", "")

	// Write multi-profile format
	setupMultiProfileFile(t, tmpDir, map[string]*Config{
		"default": {APIKey: "k1", APISecret: "s1"},
	}, "default")

	// Read the file content before Load
	cfgPath := filepath.Join(tmpDir, configDir, configFile)
	before, _ := os.ReadFile(cfgPath)

	// Load should not modify the file
	_, _ = Load(nil)

	after, _ := os.ReadFile(cfgPath)
	if string(before) != string(after) {
		t.Error("Load should not modify the credentials file")
	}
}

func TestDetectAndLoad_EmptyJSON(t *testing.T) {
	_, err := detectAndLoad([]byte(`{}`))
	if err == nil {
		t.Error("expected error for empty JSON object")
	}
}

func TestDetectAndLoad_NullJSON(t *testing.T) {
	_, err := detectAndLoad([]byte(`null`))
	if err == nil {
		t.Error("expected error for null JSON")
	}
}

func TestDetectAndLoad_MultiProfile(t *testing.T) {
	data := []byte(`{"profiles":{"prod":{"api_key":"pk","api_secret":"ps"}},"active_profile":"prod"}`)
	cf, err := detectAndLoad(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cf.ActiveProfile != "prod" {
		t.Errorf("ActiveProfile: got %q, want %q", cf.ActiveProfile, "prod")
	}
	if cf.Profiles["prod"].APIKey != "pk" {
		t.Errorf("prod APIKey: got %q", cf.Profiles["prod"].APIKey)
	}
}

func TestDetectAndLoad_FlatFormat(t *testing.T) {
	data := []byte(`{"api_key":"flat-k","api_secret":"flat-s"}`)
	cf, err := detectAndLoad(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cf.ActiveProfile != DefaultProfile {
		t.Errorf("ActiveProfile: got %q, want %q", cf.ActiveProfile, DefaultProfile)
	}
	if cf.Profiles[DefaultProfile].APIKey != "flat-k" {
		t.Errorf("default APIKey: got %q", cf.Profiles[DefaultProfile].APIKey)
	}
}

func TestDetectAndLoad_MultiProfileNoActiveProfile(t *testing.T) {
	data := []byte(`{"profiles":{"myprofile":{"api_key":"k","api_secret":"s"}}}`)
	cf, err := detectAndLoad(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// inferActiveProfile picks the only available profile
	if cf.ActiveProfile != "myprofile" {
		t.Errorf("ActiveProfile should be inferred to %q, got %q", "myprofile", cf.ActiveProfile)
	}
}

func TestDetectAndLoad_MultiProfileNoActiveProfile_PrefersDefault(t *testing.T) {
	data := []byte(`{"profiles":{"staging":{"api_key":"s","api_secret":"s"},"default":{"api_key":"d","api_secret":"d"}}}`)
	cf, err := detectAndLoad(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cf.ActiveProfile != DefaultProfile {
		t.Errorf("ActiveProfile should prefer %q when available, got %q", DefaultProfile, cf.ActiveProfile)
	}
}

func TestActiveProfileName_NoFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	name, err := ActiveProfileName()
	if err == nil {
		t.Log("no error expected when file doesn't exist (returns default)")
	}
	if name != DefaultProfile {
		t.Errorf("ActiveProfileName should return %q when no file, got %q", DefaultProfile, name)
	}
}

// --- Review-driven coverage gap tests ---

func TestLoad_NilProfileValueNoPanic(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("SOLACTL_API_KEY", "")
	t.Setenv("SOLACTL_API_SECRET", "")

	cfgDir := filepath.Join(tmpDir, configDir)
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write credentials with null profile value
	if err := os.WriteFile(
		filepath.Join(cfgDir, configFile),
		[]byte(`{"profiles":{"default":null},"active_profile":"default"}`),
		0600,
	); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Must not panic
	cfg, _ := Load(nil)
	if cfg == nil {
		t.Fatal("Load should return non-nil Config even for null profile")
	}
}

func TestListProfiles_NilProfileValueSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfgDir := filepath.Join(tmpDir, configDir)
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(cfgDir, configFile),
		[]byte(`{"profiles":{"good":{"api_key":"k","api_secret":"s"},"bad":null},"active_profile":"good"}`),
		0600,
	); err != nil {
		t.Fatalf("write: %v", err)
	}

	profiles, err := ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	// "bad" profile should be skipped
	if len(profiles) != 1 {
		t.Errorf("expected 1 profile (nil skipped), got %d", len(profiles))
	}
	if profiles[0].Name != "good" {
		t.Errorf("expected 'good' profile, got %q", profiles[0].Name)
	}
}

func TestLoad_MissingProfileWithEnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("SOLACTL_API_KEY", "env-key")
	t.Setenv("SOLACTL_API_SECRET", "env-secret")

	setupMultiProfileFile(t, tmpDir, map[string]*Config{
		"default": {APIKey: "d-key", APISecret: "d-secret"},
	}, "default")

	// Named profile doesn't exist, but env vars provide credentials
	cfg, err := Load(&LoadOptions{ProfileName: "nonexistent"})
	if err != nil {
		t.Fatalf("Load should not fail when env vars provide credentials: %v", err)
	}
	if cfg.APIKey != "env-key" {
		t.Errorf("APIKey: got %q, want %q (env should be used)", cfg.APIKey, "env-key")
	}
}

func TestLoad_MissingProfileWithFlagOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("SOLACTL_API_KEY", "")
	t.Setenv("SOLACTL_API_SECRET", "")

	setupMultiProfileFile(t, tmpDir, map[string]*Config{
		"default": {APIKey: "d-key", APISecret: "d-secret"},
	}, "default")

	cfg, err := Load(&LoadOptions{
		ProfileName: "nonexistent",
		Overrides:   &Config{APIKey: "flag-key", APISecret: "flag-secret"},
	})
	if err != nil {
		t.Fatalf("Load should not fail when overrides provide credentials: %v", err)
	}
	if cfg.APIKey != "flag-key" {
		t.Errorf("APIKey: got %q, want %q (flag should be used)", cfg.APIKey, "flag-key")
	}
}

func TestSave_RejectsCorruptFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfgDir := filepath.Join(tmpDir, configDir)
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write corrupt (non-JSON) content
	if err := os.WriteFile(filepath.Join(cfgDir, configFile), []byte("NOT JSON"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := Save(&Config{APIKey: "k", APISecret: "s"}, "")
	if err == nil {
		t.Fatal("Save should fail when existing file is corrupt (to prevent data loss)")
	}
	if !strings.Contains(err.Error(), "설정 파일 읽기 실패") {
		t.Errorf("error should mention file read failure, got: %v", err)
	}
}

func TestValidateProfileName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "default", false},
		{"valid with hyphen", "my-profile", false},
		{"valid with underscore", "my_profile", false},
		{"valid with numbers", "profile123", false},
		{"empty", "", true},
		{"too long", strings.Repeat("a", 65), true},
		{"max length", strings.Repeat("a", 64), false},
		{"space", "my profile", true},
		{"slash", "my/profile", true},
		{"dot", "my.profile", true},
		{"unicode", "프로필", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProfileName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProfileName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestSave_RejectsInvalidProfileName(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	err := Save(&Config{APIKey: "k", APISecret: "s"}, "bad name!")
	if err == nil {
		t.Fatal("Save should reject invalid profile name")
	}
}

func TestInferActiveProfile_PointsToMissing(t *testing.T) {
	cf := &CredentialsFile{
		Profiles: map[string]*Config{
			"staging": {APIKey: "s", APISecret: "s"},
			"prod":    {APIKey: "p", APISecret: "p"},
		},
		ActiveProfile: "deleted-profile",
	}
	inferActiveProfile(cf)
	// Should pick first alphabetically since "default" doesn't exist
	if cf.ActiveProfile != "prod" {
		t.Errorf("ActiveProfile should be inferred to %q, got %q", "prod", cf.ActiveProfile)
	}
}

func TestInferActiveProfile_PrefersDefault(t *testing.T) {
	cf := &CredentialsFile{
		Profiles: map[string]*Config{
			"staging": {APIKey: "s", APISecret: "s"},
			"default": {APIKey: "d", APISecret: "d"},
		},
		ActiveProfile: "",
	}
	inferActiveProfile(cf)
	if cf.ActiveProfile != DefaultProfile {
		t.Errorf("ActiveProfile should prefer %q, got %q", DefaultProfile, cf.ActiveProfile)
	}
}

func FuzzCredentialsFileJSON(f *testing.F) {
	f.Add([]byte(`{"profiles":{"default":{"api_key":"k","api_secret":"s"}},"active_profile":"default"}`))
	f.Add([]byte(`{"api_key":"k","api_secret":"s"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(``))
	f.Add([]byte(`{"profiles":null}`))
	f.Add([]byte(`{"profiles":{}}`))
	f.Add([]byte{0xff, 0xfe, 0xfd})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must not panic regardless of input
		_, _ = detectAndLoad(data)
	})
}
