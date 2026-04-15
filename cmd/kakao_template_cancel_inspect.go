package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var kakaoTemplateCancelInspectCmd = &cobra.Command{
	Use:   "cancel-inspect <templateId>",
	Short: "알림톡 템플릿 검수를 취소합니다",
	Long: `알림톡 템플릿의 검수 요청을 취소합니다.

사용법:
  solactl kakao template cancel-inspect KA01TP240101000000000000000000000`,
	Args: cobra.ExactArgs(1),
	RunE: runKakaoTemplateCancelInspect,
}

func init() {
	kakaoTemplateCmd.AddCommand(kakaoTemplateCancelInspectCmd)
}

func runKakaoTemplateCancelInspect(cmd *cobra.Command, args []string) error {
	templateID := args[0]

	c, err := newClient()
	if err != nil {
		return err
	}

	raw, err := c.Put(ctx(), fmt.Sprintf("kakao/v2/templates/%s/inspection/cancel", templateID), nil)
	if err != nil {
		return fmt.Errorf("알림톡 템플릿 검수 취소 실패: %w", err)
	}

	p := printer()
	if flagJSON {
		return p.PrintJSON(raw)
	}

	_, _ = fmt.Fprintf(out(), "알림톡 템플릿 %s의 검수가 취소되었습니다.\n", templateID)
	return nil
}
