package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/types"
)

var (
	kakaoTplSendableFlagChannelID    string
	kakaoTplSendableFlagTemplateID   string
	kakaoTplSendableFlagSenderKey    string
	kakaoTplSendableFlagTemplateCode string
)

var kakaoTemplateSendableCmd = &cobra.Command{
	Use:   "sendable",
	Short: "발송 가능한 알림톡 템플릿 목록을 조회합니다",
	Long: `발송 가능한 알림톡 템플릿 목록을 조회합니다.

사용법:
  solactl kakao template sendable
  solactl kakao template sendable --channel-id <channelId>`,
	RunE: runKakaoTemplateSendable,
}

func init() {
	kakaoTemplateSendableCmd.Flags().StringVar(&kakaoTplSendableFlagChannelID, "channel-id", "", "채널 ID 필터")
	kakaoTemplateSendableCmd.Flags().StringVar(&kakaoTplSendableFlagTemplateID, "template-id", "", "템플릿 ID 필터")
	kakaoTemplateSendableCmd.Flags().StringVar(&kakaoTplSendableFlagSenderKey, "sender-key", "", "발신 키 필터")
	kakaoTemplateSendableCmd.Flags().StringVar(&kakaoTplSendableFlagTemplateCode, "template-code", "", "템플릿 코드 필터")
	kakaoTemplateCmd.AddCommand(kakaoTemplateSendableCmd)
}

func runKakaoTemplateSendable(cmd *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}

	params := url.Values{}
	if kakaoTplSendableFlagChannelID != "" {
		params.Set("channelId", kakaoTplSendableFlagChannelID)
	}
	if kakaoTplSendableFlagTemplateID != "" {
		params.Set("templateId", kakaoTplSendableFlagTemplateID)
	}
	if kakaoTplSendableFlagSenderKey != "" {
		params.Set("senderKey", kakaoTplSendableFlagSenderKey)
	}
	if kakaoTplSendableFlagTemplateCode != "" {
		params.Set("templateCode", kakaoTplSendableFlagTemplateCode)
	}

	raw, err := c.Get(ctx(), "kakao/v2/templates/sendable", params)
	if err != nil {
		return fmt.Errorf("발송 가능 템플릿 조회 실패: %w", err)
	}

	p := printer()
	if flagJSON {
		return p.PrintJSON(raw)
	}

	var templates []types.KakaoTemplate
	if err := json.Unmarshal(raw, &templates); err != nil {
		return fmt.Errorf("응답 파싱 실패: %w", err)
	}

	headers := []string{"TEMPLATE ID", "NAME", "STATUS", "TYPE", "CHANNEL ID"}
	rows := make([][]string, 0, len(templates))
	for i := range templates {
		tpl := &templates[i]
		rows = append(rows, []string{
			tpl.TemplateID,
			tpl.Name,
			types.DisplayStatus(tpl.Status),
			tpl.DisplayMessageType(),
			tpl.DisplayChannelRef(),
		})
	}
	p.FormatTable(headers, rows)

	return nil
}
