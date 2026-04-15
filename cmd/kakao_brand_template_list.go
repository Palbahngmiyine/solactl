package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/types"
)

var (
	kakaoBtplListFlagName            string
	kakaoBtplListFlagChannelID       string
	kakaoBtplListFlagChannelGroupID  string
	kakaoBtplListFlagBrandTemplateID string
	kakaoBtplListFlagChatBubbleType  string
	kakaoBtplListFlagStatus          string
	kakaoBtplListFlagStartKey        string
	kakaoBtplListFlagLimit           int
)

var kakaoBrandTemplateListCmd = &cobra.Command{
	Use:   "list",
	Short: "브랜드 템플릿 목록을 조회합니다",
	Long: `브랜드 템플릿 목록을 조회합니다.

사용법:
  solactl kakao brand-template list
  solactl kakao brand-template list --chat-bubble-type TEXT
  solactl kakao brand-template list --start-key <nextKey>`,
	RunE: runKakaoBrandTemplateList,
}

func init() {
	kakaoBrandTemplateListCmd.Flags().StringVar(&kakaoBtplListFlagName, "name", "", "이름 필터")
	kakaoBrandTemplateListCmd.Flags().StringVar(&kakaoBtplListFlagChannelID, "channel-id", "", "채널 ID 필터")
	kakaoBrandTemplateListCmd.Flags().StringVar(&kakaoBtplListFlagChannelGroupID, "channel-group-id", "", "채널 그룹 ID 필터")
	kakaoBrandTemplateListCmd.Flags().StringVar(&kakaoBtplListFlagBrandTemplateID, "brand-template-id", "", "브랜드 템플릿 ID 필터")
	kakaoBrandTemplateListCmd.Flags().StringVar(&kakaoBtplListFlagChatBubbleType, "chat-bubble-type", "", "말풍선 타입 필터")
	kakaoBrandTemplateListCmd.Flags().StringVar(&kakaoBtplListFlagStatus, "status", "", "상태 필터 (ACTIVE/INACTIVE/DELETED)")
	kakaoBrandTemplateListCmd.Flags().StringVar(&kakaoBtplListFlagStartKey, "start-key", "", "페이지네이션 시작 키")
	kakaoBrandTemplateListCmd.Flags().IntVar(&kakaoBtplListFlagLimit, "limit", 20, "조회 건수")
	kakaoBrandTemplateCmd.AddCommand(kakaoBrandTemplateListCmd)
}

func runKakaoBrandTemplateList(cmd *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}

	params := url.Values{}
	params.Set("limit", strconv.Itoa(kakaoBtplListFlagLimit))
	if kakaoBtplListFlagName != "" {
		params.Set("name", kakaoBtplListFlagName)
	}
	// brand-template API uses pfId/pfGroupId directly
	if kakaoBtplListFlagChannelID != "" {
		params.Set("pfId", kakaoBtplListFlagChannelID)
	}
	if kakaoBtplListFlagChannelGroupID != "" {
		params.Set("pfGroupId", kakaoBtplListFlagChannelGroupID)
	}
	if kakaoBtplListFlagBrandTemplateID != "" {
		params.Set("brandTemplateId", kakaoBtplListFlagBrandTemplateID)
	}
	if kakaoBtplListFlagChatBubbleType != "" {
		params.Set("chatBubbleType", kakaoBtplListFlagChatBubbleType)
	}
	if kakaoBtplListFlagStatus != "" {
		params.Set("status", kakaoBtplListFlagStatus)
	}
	if kakaoBtplListFlagStartKey != "" {
		params.Set("startKey", kakaoBtplListFlagStartKey)
	}

	raw, err := c.Get(ctx(), "kakao/v2/brand-templates", params)
	if err != nil {
		return fmt.Errorf("브랜드 템플릿 목록 조회 실패: %w", err)
	}

	p := printer()
	if flagJSON {
		return p.PrintJSON(raw)
	}

	var resp types.KakaoBrandTemplateListResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("응답 파싱 실패: %w", err)
	}

	headers := []string{"BRAND TEMPLATE ID", "NAME", "BUBBLE TYPE", "STATUS", "CREATED"}
	rows := make([][]string, 0, len(resp.BrandTemplateList))
	for i := range resp.BrandTemplateList {
		bt := &resp.BrandTemplateList[i]
		rows = append(rows, []string{
			bt.BrandTemplateID,
			bt.Name,
			bt.ChatBubbleType,
			types.DisplayStatus(bt.Status),
			types.DisplayDate(bt.DateCreated),
		})
	}
	p.FormatTable(headers, rows)

	if resp.NextKey != "" {
		_, _ = fmt.Fprintf(errOut(), "\n다음 페이지: solactl kakao brand-template list --start-key %s\n", resp.NextKey)
	}

	return nil
}
