package cmd

import "github.com/spf13/cobra"

var messagesCmd = &cobra.Command{
	Use:     "messages",
	Aliases: []string{"msg"},
	Short:   "메시지를 조회합니다",
}

func init() {
	rootCmd.AddCommand(messagesCmd)
}
