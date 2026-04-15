package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/config"
)

func init() {
	configureCmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "현재 설정을 표시합니다",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(&config.LoadOptions{ProfileName: flagProfile})
			if err != nil {
				return err
			}

			profileName := flagProfile
			if profileName == "" {
				profileName, _ = config.ActiveProfileName()
			}

			path, _ := config.ConfigFilePath()
			p := printer()
			p.PrintKeyValue(
				"Config File", path,
				"Profile", profileName,
				"API Key", cfg.APIKey,
				"API Secret", config.MaskSecret(cfg.APISecret),
			)

			if err := cfg.Validate(); err != nil {
				_, _ = fmt.Fprintf(errOut(), "\n⚠ %v\n", err)
			}
			return nil
		},
	})
}
