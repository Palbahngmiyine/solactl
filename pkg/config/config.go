package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const (
	configDir      = ".solactl"
	configFile     = "credentials.json"
	DefaultProfile = "default"
)

// Config holds the CLI configuration for a single profile.
type Config struct {
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
}

// CredentialsFile is the on-disk multi-profile credentials format.
type CredentialsFile struct {
	Profiles      map[string]*Config `json:"profiles"`
	ActiveProfile string             `json:"active_profile"`
}

// LoadOptions controls profile selection during Load.
type LoadOptions struct {
	Overrides   *Config
	ProfileName string
}

// ProfileInfo holds profile metadata for listing.
type ProfileInfo struct {
	Name   string
	Config *Config
	Active bool
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
// Priority: overrides > env vars > named profile > active profile.
func Load(opts *LoadOptions) (*Config, error) {
	cfg := &Config{}

	// 1. Load from file (active or named profile)
	profileName := ""
	if opts != nil {
		profileName = opts.ProfileName
	}

	fileCfg, err := loadProfileFromFile(profileName)
	if err != nil && profileName != "" {
		return nil, err
	}
	if fileCfg != nil {
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
	if opts != nil && opts.Overrides != nil {
		if opts.Overrides.APIKey != "" {
			cfg.APIKey = opts.Overrides.APIKey
		}
		if opts.Overrides.APISecret != "" {
			cfg.APISecret = opts.Overrides.APISecret
		}
	}

	return cfg, nil
}

// loadProfileFromFile loads a specific profile from the credentials file.
// If profileName is empty, the active profile is used.
func loadProfileFromFile(profileName string) (*Config, error) {
	cf, err := loadCredentialsFile()
	if err != nil {
		return nil, err
	}

	name := profileName
	if name == "" {
		name = cf.ActiveProfile
	}
	if name == "" {
		name = DefaultProfile
	}

	profile, ok := cf.Profiles[name]
	if !ok {
		if profileName != "" {
			return nil, fmt.Errorf("프로필 '%s'을(를) 찾을 수 없습니다", profileName)
		}
		return nil, fmt.Errorf("활성 프로필이 없습니다")
	}
	return profile, nil
}

// Save writes the config to a specific profile in ~/.solactl/credentials.json.
// If profileName is empty, it uses "default".
func Save(cfg *Config, profileName string) error {
	if profileName == "" {
		profileName = DefaultProfile
	}

	cf, _ := loadCredentialsFile()
	if cf == nil {
		cf = &CredentialsFile{Profiles: make(map[string]*Config)}
	}
	if cf.Profiles == nil {
		cf.Profiles = make(map[string]*Config)
	}

	cf.Profiles[profileName] = cfg

	// Set active profile if this is the first profile or no active profile set
	if cf.ActiveProfile == "" {
		cf.ActiveProfile = profileName
	}

	return saveCredentialsFile(cf)
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

// LoadFromFile reads the active profile config from the credentials file only,
// without merging environment variables or applying overrides.
func LoadFromFile() (*Config, error) {
	return loadProfileFromFile("")
}

// LoadCredentialsFile reads the raw multi-profile file structure.
func LoadCredentialsFile() (*CredentialsFile, error) {
	return loadCredentialsFile()
}

// SaveCredentialsFile writes the entire credentials file to disk.
func SaveCredentialsFile(cf *CredentialsFile) error {
	return saveCredentialsFile(cf)
}

// loadCredentialsFile reads and parses the credentials file.
// It detects old flat format and converts to multi-profile format.
func loadCredentialsFile() (*CredentialsFile, error) {
	dir, err := configDirPath()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, configFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return detectAndLoad(data)
}

// detectAndLoad parses credentials data, auto-detecting old flat format.
func detectAndLoad(data []byte) (*CredentialsFile, error) {
	// Try new multi-profile format first
	var cf CredentialsFile
	if err := json.Unmarshal(data, &cf); err == nil && cf.Profiles != nil && len(cf.Profiles) > 0 {
		if cf.ActiveProfile == "" {
			cf.ActiveProfile = DefaultProfile
		}
		return &cf, nil
	}

	// Try old flat format (backward compatibility)
	var old Config
	if err := json.Unmarshal(data, &old); err == nil && (old.APIKey != "" || old.APISecret != "") {
		return &CredentialsFile{
			Profiles:      map[string]*Config{DefaultProfile: &old},
			ActiveProfile: DefaultProfile,
		}, nil
	}

	return nil, fmt.Errorf("설정 파일 파싱 실패")
}

func saveCredentialsFile(cf *CredentialsFile) error {
	dir, err := configDirPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("디렉토리 생성 실패: %w", err)
	}

	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON 직렬화 실패: %w", err)
	}

	path := filepath.Join(dir, configFile)
	return os.WriteFile(path, data, 0600)
}

// ActiveProfileName returns the name of the currently active profile.
func ActiveProfileName() (string, error) {
	cf, err := loadCredentialsFile()
	if err != nil {
		return DefaultProfile, err
	}
	if cf.ActiveProfile == "" {
		return DefaultProfile, nil
	}
	return cf.ActiveProfile, nil
}

// SetActiveProfile updates the active_profile field in the credentials file.
func SetActiveProfile(name string) error {
	cf, err := loadCredentialsFile()
	if err != nil {
		return fmt.Errorf("설정 파일 읽기 실패: %w", err)
	}

	if _, ok := cf.Profiles[name]; !ok {
		return fmt.Errorf("프로필 '%s'을(를) 찾을 수 없습니다", name)
	}

	cf.ActiveProfile = name
	return saveCredentialsFile(cf)
}

// DeleteProfile removes a profile from the credentials file.
func DeleteProfile(name string) error {
	cf, err := loadCredentialsFile()
	if err != nil {
		return fmt.Errorf("설정 파일 읽기 실패: %w", err)
	}

	if _, ok := cf.Profiles[name]; !ok {
		return fmt.Errorf("프로필 '%s'을(를) 찾을 수 없습니다", name)
	}

	if cf.ActiveProfile == name {
		return fmt.Errorf("활성 프로필 '%s'은(는) 삭제할 수 없습니다. 먼저 다른 프로필로 전환하세요", name)
	}

	if len(cf.Profiles) <= 1 {
		return fmt.Errorf("마지막 프로필은 삭제할 수 없습니다")
	}

	delete(cf.Profiles, name)
	return saveCredentialsFile(cf)
}

// ListProfiles returns all profile names and indicates which is active.
func ListProfiles() ([]ProfileInfo, error) {
	cf, err := loadCredentialsFile()
	if err != nil {
		return nil, err
	}

	var profiles []ProfileInfo
	for name, cfg := range cf.Profiles {
		profiles = append(profiles, ProfileInfo{
			Name:   name,
			Config: cfg,
			Active: name == cf.ActiveProfile,
		})
	}

	// Sort by name for deterministic output
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})

	return profiles, nil
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
