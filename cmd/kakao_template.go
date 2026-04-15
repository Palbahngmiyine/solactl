package cmd

import "github.com/spf13/cobra"

var kakaoTemplateCmd = &cobra.Command{
	Use:     "template",
	Aliases: []string{"tpl"},
	Short:   "카카오 알림톡 템플릿을 관리합니다",
}

func init() {
	kakaoCmd.AddCommand(kakaoTemplateCmd)
}
