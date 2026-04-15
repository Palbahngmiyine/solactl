package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/types"
)

var kakaoTemplateGetCmd = &cobra.Command{
	Use:   "get <templateId>",
	Short: "알림톡 템플릿 상세 정보를 조회합니다",
	Long: `알림톡 템플릿의 상세 정보를 조회합니다.

사용법:
  solactl kakao template get KA01TP240101000000000000000000000`,
	Args: cobra.ExactArgs(1),
	RunE: runKakaoTemplateGet,
}

func init() {
	kakaoTemplateCmd.AddCommand(kakaoTemplateGetCmd)
}

func runKakaoTemplateGet(cmd *cobra.Command, args []string) error {
	templateID := args[0]

	c, err := newClient()
	if err != nil {
		return err
	}

	raw, err := c.Get(ctx(), fmt.Sprintf("kakao/v2/templates/%s", templateID), nil)
	if err != nil {
		return fmt.Errorf("알림톡 템플릿 조회 실패: %w", err)
	}

	p := printer()
	if flagJSON {
		return p.PrintJSON(raw)
	}

	var tpl types.KakaoTemplate
	if err := json.Unmarshal(raw, &tpl); err != nil {
		return fmt.Errorf("응답 파싱 실패: %w", err)
	}

	p.PrintKeyValue(
		"Template ID", tpl.TemplateID,
		"Name", tpl.Name,
		"Status", types.DisplayStatus(tpl.Status),
		"Message Type", tpl.DisplayMessageType(),
		"Emphasize Type", types.DisplayStatus(tpl.EmphasizeType),
		"Channel", tpl.DisplayChannelRef(),
		"Category Code", tpl.CategoryCode,
		"Content", tpl.Content,
		"Hidden", types.DisplayBool(tpl.IsHidden),
		"Security Flag", types.DisplayBool(tpl.SecurityFlag),
		"Account ID", tpl.AccountID,
		"Created", types.DisplayDate(tpl.DateCreated),
		"Updated", types.DisplayDate(tpl.DateUpdated),
	)

	return nil
}
