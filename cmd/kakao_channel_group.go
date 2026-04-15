package cmd

import "github.com/spf13/cobra"

var kakaoChannelGroupCmd = &cobra.Command{
	Use:     "channel-group",
	Aliases: []string{"chg"},
	Short:   "카카오톡 채널 그룹을 조회합니다",
}

func init() {
	kakaoCmd.AddCommand(kakaoChannelGroupCmd)
}
