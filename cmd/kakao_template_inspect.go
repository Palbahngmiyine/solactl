package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var kakaoTplInspectFlagComment string

var kakaoTemplateInspectCmd = &cobra.Command{
	Use:   "inspect <templateId>",
	Short: "알림톡 템플릿 검수를 요청합니다",
	Long: `알림톡 템플릿의 검수를 요청합니다.

사용법:
  solactl kakao template inspect KA01TP240101000000000000000000000
  solactl kakao template inspect KA01TP240101000000000000000000000 --comment "검수 부탁드립니다"`,
	Args: cobra.ExactArgs(1),
	RunE: runKakaoTemplateInspect,
}

func init() {
	kakaoTemplateInspectCmd.Flags().StringVar(&kakaoTplInspectFlagComment, "comment", "", "검수 코멘트")
	kakaoTemplateCmd.AddCommand(kakaoTemplateInspectCmd)
}

func runKakaoTemplateInspect(cmd *cobra.Command, args []string) error {
	templateID := args[0]

	c, err := newClient()
	if err != nil {
		return err
	}

	body := map[string]interface{}{}
	if kakaoTplInspectFlagComment != "" {
		body["comment"] = kakaoTplInspectFlagComment
	}

	raw, err := c.Put(ctx(), fmt.Sprintf("kakao/v2/templates/%s/inspection", templateID), body)
	if err != nil {
		return fmt.Errorf("알림톡 템플릿 검수 요청 실패: %w", err)
	}

	p := printer()
	if flagJSON {
		return p.PrintJSON(raw)
	}

	_, _ = fmt.Fprintf(out(), "알림톡 템플릿 %s의 검수가 요청되었습니다.\n", templateID)
	return nil
}
