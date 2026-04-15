package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/config"
)

func init() {
	configureCmd.AddCommand(&cobra.Command{
		Use:   "delete <profile>",
		Short: "저장된 프로필을 삭제합니다",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]

			if err := config.DeleteProfile(profileName); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(errOut(), "프로필 '%s'이(가) 삭제되었습니다.\n", profileName)
			return nil
		},
	})
}
