package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/internal/version"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "버전 정보를 표시합니다",
		Run: func(cmd *cobra.Command, args []string) {
			_, _ = fmt.Fprintf(out(), "solactl %s (commit: %s, date: %s)\n", version.Version, version.Commit, version.Date)
		},
	})
}
