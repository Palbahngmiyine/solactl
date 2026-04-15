package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/config"
)

func init() {
	configureCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "저장된 프로필 목록을 표시합니다",
		RunE: func(cmd *cobra.Command, args []string) error {
			profiles, err := config.ListProfiles()
			if err != nil {
				if os.IsNotExist(err) {
					_, _ = fmt.Fprintf(errOut(), "프로필이 없습니다. 'solactl configure'를 실행하세요.\n")
					return nil
				}
				return fmt.Errorf("프로필 목록 조회 실패: %w", err)
			}

			if len(profiles) == 0 {
				_, _ = fmt.Fprintf(errOut(), "프로필이 없습니다. 'solactl configure'를 실행하세요.\n")
				return nil
			}

			p := printer()
			for _, prof := range profiles {
				active := ""
				if prof.Active {
					active = " *"
				}
				p.PrintKeyValue(
					"Profile", prof.Name+active,
					"API Key", prof.Config.APIKey,
					"API Secret", config.MaskSecret(prof.Config.APISecret),
				)
				_, _ = fmt.Fprintln(out())
			}
			return nil
		},
	})
}
