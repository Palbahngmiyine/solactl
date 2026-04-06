package cmd

import "github.com/spf13/cobra"

var senderidCmd = &cobra.Command{
	Use:     "senderid",
	Aliases: []string{"sid"},
	Short:   "발신번호를 관리합니다",
}

func init() {
	rootCmd.AddCommand(senderidCmd)
}
