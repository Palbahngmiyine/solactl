package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/types"
)

var (
	kakaoBtplSendableFlagChannelID       string
	kakaoBtplSendableFlagBrandTemplateID string
)

var kakaoBrandTemplateSendableCmd = &cobra.Command{
	Use:   "sendable",
	Short: "발송 가능한 브랜드 템플릿 목록을 조회합니다",
	Long: `발송 가능한 브랜드 템플릿 목록을 조회합니다.

사용법:
  solactl kakao brand-template sendable
  solactl kakao brand-template sendable --channel-id <channelId>`,
	RunE: runKakaoBrandTemplateSendable,
}

func init() {
	kakaoBrandTemplateSendableCmd.Flags().StringVar(&kakaoBtplSendableFlagChannelID, "channel-id", "", "채널 ID 필터")
	kakaoBrandTemplateSendableCmd.Flags().StringVar(&kakaoBtplSendableFlagBrandTemplateID, "brand-template-id", "", "브랜드 템플릿 ID 필터")
	kakaoBrandTemplateCmd.AddCommand(kakaoBrandTemplateSendableCmd)
}

func runKakaoBrandTemplateSendable(cmd *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}

	params := url.Values{}
	// brand-template API uses pfId directly
	if kakaoBtplSendableFlagChannelID != "" {
		params.Set("pfId", kakaoBtplSendableFlagChannelID)
	}
	if kakaoBtplSendableFlagBrandTemplateID != "" {
		params.Set("brandTemplateId", kakaoBtplSendableFlagBrandTemplateID)
	}

	raw, err := c.Get(ctx(), "kakao/v2/brand-templates/sendable", params)
	if err != nil {
		return fmt.Errorf("발송 가능 브랜드 템플릿 조회 실패: %w", err)
	}

	p := printer()
	if flagJSON {
		return p.PrintJSON(raw)
	}

	var templates []types.KakaoBrandTemplate
	if err := json.Unmarshal(raw, &templates); err != nil {
		return fmt.Errorf("응답 파싱 실패: %w", err)
	}

	headers := []string{"BRAND TEMPLATE ID", "NAME", "BUBBLE TYPE", "STATUS"}
	rows := make([][]string, 0, len(templates))
	for i := range templates {
		bt := &templates[i]
		rows = append(rows, []string{
			bt.BrandTemplateID,
			bt.Name,
			bt.ChatBubbleType,
			types.DisplayStatus(bt.Status),
		})
	}
	p.FormatTable(headers, rows)

	return nil
}
