package cmd

import "github.com/spf13/cobra"

var kakaoCmd = &cobra.Command{
	Use:   "kakao",
	Short: "카카오톡 채널, 템플릿을 관리합니다",
}

func init() {
	rootCmd.AddCommand(kakaoCmd)
}
