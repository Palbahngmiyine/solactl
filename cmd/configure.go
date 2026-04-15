package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/config"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "API Key와 Secret을 설정합니다",
	Long: `API Key와 API Secret을 설정합니다.

대화형 모드:
  solactl configure
  solactl configure --profile staging

비대화형 모드:
  solactl configure --api-key <key> --api-secret <secret>
  solactl configure --profile staging --api-key <key> --api-secret <secret>

API Key는 https://console.solapi.com 에서 발급받을 수 있습니다.`,
	RunE: runConfigure,
}

func init() {
	rootCmd.AddCommand(configureCmd)
}

func runConfigure(cmd *cobra.Command, args []string) error {
	apiKey := flagAPIKey
	apiSecret := flagAPISecret

	// Determine target profile name
	profileName := flagProfile
	if profileName == "" {
		profileName, _ = config.ActiveProfileName()
	}

	// Non-interactive mode if both key and secret are provided via flags
	if apiKey != "" && apiSecret != "" {
		return saveConfigure(&config.Config{APIKey: apiKey, APISecret: apiSecret}, profileName)
	}

	// Interactive mode
	reader := bufio.NewReader(os.Stdin)

	_, _ = fmt.Fprintln(out(), "solactl 초기 설정")
	_, _ = fmt.Fprintln(out(), "API Key는 https://console.solapi.com 에서 발급받을 수 있습니다.")
	_, _ = fmt.Fprintf(errOut(), "프로필: %s\n", profileName)
	_, _ = fmt.Fprintln(out())

	// Load existing config for defaults
	existing, _ := config.Load(&config.LoadOptions{ProfileName: profileName})

	// API Key
	if apiKey == "" {
		prompt := "API Key"
		if existing != nil && existing.APIKey != "" {
			prompt += fmt.Sprintf(" [%s]", existing.APIKey)
		}
		_, _ = fmt.Fprintf(out(), "%s: ", prompt)
		line, err := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" && err != nil {
			return fmt.Errorf("입력이 중단되었습니다")
		}
		if line != "" {
			apiKey = line
		} else if existing != nil {
			apiKey = existing.APIKey
		}
	}

	if apiKey == "" {
		return fmt.Errorf("API Key를 입력하세요")
	}

	// API Secret
	if apiSecret == "" {
		prompt := "API Secret"
		if existing != nil && existing.APISecret != "" {
			prompt += fmt.Sprintf(" [%s]", config.MaskSecret(existing.APISecret))
		}
		_, _ = fmt.Fprintf(out(), "%s: ", prompt)
		line, err := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" && err != nil {
			return fmt.Errorf("입력이 중단되었습니다")
		}
		if line != "" {
			apiSecret = line
		} else if existing != nil {
			apiSecret = existing.APISecret
		}
	}

	if apiSecret == "" {
		return fmt.Errorf("API Secret을 입력하세요")
	}

	return saveConfigure(&config.Config{APIKey: apiKey, APISecret: apiSecret}, profileName)
}

func saveConfigure(cfg *config.Config, profileName string) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	if err := config.Save(cfg, profileName); err != nil {
		return fmt.Errorf("설정 저장 실패: %w", err)
	}

	path, _ := config.ConfigFilePath()
	_, _ = fmt.Fprintf(out(), "설정이 저장되었습니다: %s\n", path)
	_, _ = fmt.Fprintf(out(), "  프로필:     %s\n", profileName)
	_, _ = fmt.Fprintf(out(), "  API Key:    %s\n", cfg.APIKey)
	_, _ = fmt.Fprintf(out(), "  API Secret: %s\n", config.MaskSecret(cfg.APISecret))
	return nil
}
