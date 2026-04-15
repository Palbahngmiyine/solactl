package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/config"
)

func init() {
	configureCmd.AddCommand(&cobra.Command{
		Use:   "use <profile>",
		Short: "활성 프로필을 전환합니다",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]

			if err := config.SetActiveProfile(profileName); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(errOut(), "활성 프로필이 '%s'(으)로 전환되었습니다.\n", profileName)
			return nil
		},
	})
}
