package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	configDir  = ".solactl"
	configFile = "credentials.json"
)

// Config holds the CLI configuration.
type Config struct {
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
}

// Validate checks that the config has non-empty credentials.
func (c *Config) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("API Key가 설정되지 않았습니다. 'solactl configure'를 실행하세요")
	}
	if c.APISecret == "" {
		return fmt.Errorf("API Secret이 설정되지 않았습니다. 'solactl configure'를 실행하세요")
	}
	return nil
}

// Load reads config from file, env vars, and applies overrides.
// Priority: overrides > env vars > config file.
func Load(overrides *Config) (*Config, error) {
	cfg := &Config{}

	// 1. Load from file
	if fileCfg, err := loadFromFile(); err == nil {
		mergeConfig(cfg, fileCfg)
	}

	// 2. Apply env vars
	if v := os.Getenv("SOLACTL_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("SOLACTL_API_SECRET"); v != "" {
		cfg.APISecret = v
	}

	// 3. Apply CLI flag overrides
	if overrides != nil {
		if overrides.APIKey != "" {
			cfg.APIKey = overrides.APIKey
		}
		if overrides.APISecret != "" {
			cfg.APISecret = overrides.APISecret
		}
	}

	return cfg, nil
}

// Save writes the config to ~/.solactl/credentials.json.
func Save(cfg *Config) error {
	dir, err := configDirPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("디렉토리 생성 실패: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON 직렬화 실패: %w", err)
	}

	path := filepath.Join(dir, configFile)
	return os.WriteFile(path, data, 0600)
}

// ConfigFilePath returns the path to the credentials file.
func ConfigFilePath() (string, error) {
	dir, err := configDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFile), nil
}

func configDirPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("홈 디렉토리를 찾을 수 없습니다: %w", err)
	}
	return filepath.Join(home, configDir), nil
}

// LoadFromFile reads config from the credentials file only, without merging
// environment variables or applying overrides.
func LoadFromFile() (*Config, error) {
	return loadFromFile()
}

func loadFromFile() (*Config, error) {
	dir, err := configDirPath()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, configFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("설정 파일 파싱 실패: %w", err)
	}
	return &cfg, nil
}

func mergeConfig(dst, src *Config) {
	if src.APIKey != "" {
		dst.APIKey = src.APIKey
	}
	if src.APISecret != "" {
		dst.APISecret = src.APISecret
	}
}

// MaskSecret masks all but the first 4 characters of a secret.
func MaskSecret(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[:4] + "****************************"
}
