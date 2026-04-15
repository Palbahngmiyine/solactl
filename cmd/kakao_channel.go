package cmd

import "github.com/spf13/cobra"

var kakaoChannelCmd = &cobra.Command{
	Use:     "channel",
	Aliases: []string{"ch"},
	Short:   "카카오톡 채널을 조회합니다",
}

func init() {
	kakaoCmd.AddCommand(kakaoChannelCmd)
}
