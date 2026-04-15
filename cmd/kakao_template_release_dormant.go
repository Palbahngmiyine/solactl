package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var kakaoTemplateReleaseDormantCmd = &cobra.Command{
	Use:   "release-dormant <templateId>",
	Short: "알림톡 템플릿 휴면을 해제합니다",
	Long: `알림톡 템플릿의 휴면 상태를 해제합니다.

사용법:
  solactl kakao template release-dormant KA01TP240101000000000000000000000`,
	Args: cobra.ExactArgs(1),
	RunE: runKakaoTemplateReleaseDormant,
}

func init() {
	kakaoTemplateCmd.AddCommand(kakaoTemplateReleaseDormantCmd)
}

func runKakaoTemplateReleaseDormant(cmd *cobra.Command, args []string) error {
	templateID := args[0]

	c, err := newClient()
	if err != nil {
		return err
	}

	// API path has intentional typo "relese-dormant" (matches server-side route)
	raw, err := c.Post(ctx(), fmt.Sprintf("kakao/v2/templates/%s/relese-dormant", templateID), nil)
	if err != nil {
		return fmt.Errorf("알림톡 템플릿 휴면 해제 실패: %w", err)
	}

	p := printer()
	if flagJSON {
		return p.PrintJSON(raw)
	}

	_, _ = fmt.Fprintf(out(), "알림톡 템플릿 %s의 휴면이 해제되었습니다.\n", templateID)
	return nil
}
