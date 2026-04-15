package cmd

import "github.com/spf13/cobra"

var kakaoBrandTemplateCmd = &cobra.Command{
	Use:     "brand-template",
	Aliases: []string{"btpl"},
	Short:   "카카오 브랜드 템플릿을 관리합니다",
}

func init() {
	kakaoCmd.AddCommand(kakaoBrandTemplateCmd)
}
