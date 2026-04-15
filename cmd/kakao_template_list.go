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
	kakaoTplListFlagName           string
	kakaoTplListFlagChannelID      string
	kakaoTplListFlagChannelGroupID string
	kakaoTplListFlagTemplateID     string
	kakaoTplListFlagStatus         string
	kakaoTplListFlagIsHidden       bool
	kakaoTplListFlagIsMine         bool
	kakaoTplListFlagStartKey       string
	kakaoTplListFlagLimit          int
)

var kakaoTemplateListCmd = &cobra.Command{
	Use:   "list",
	Short: "알림톡 템플릿 목록을 조회합니다",
	Long: `알림톡 템플릿 목록을 조회합니다.

사용법:
  solactl kakao template list
  solactl kakao template list --status APPROVED
  solactl kakao template list --channel-id <channelId>
  solactl kakao template list --start-key <nextKey>`,
	RunE: runKakaoTemplateList,
}

func init() {
	kakaoTemplateListCmd.Flags().StringVar(&kakaoTplListFlagName, "name", "", "템플릿 이름 필터")
	kakaoTemplateListCmd.Flags().StringVar(&kakaoTplListFlagChannelID, "channel-id", "", "채널 ID 필터")
	kakaoTemplateListCmd.Flags().StringVar(&kakaoTplListFlagChannelGroupID, "channel-group-id", "", "채널 그룹 ID 필터")
	kakaoTemplateListCmd.Flags().StringVar(&kakaoTplListFlagTemplateID, "template-id", "", "템플릿 ID 필터")
	kakaoTemplateListCmd.Flags().StringVar(&kakaoTplListFlagStatus, "status", "", "상태 필터 (PENDING/INSPECTING/APPROVED/REJECTED)")
	kakaoTemplateListCmd.Flags().BoolVar(&kakaoTplListFlagIsHidden, "is-hidden", false, "숨김 템플릿만 조회")
	kakaoTemplateListCmd.Flags().BoolVar(&kakaoTplListFlagIsMine, "is-mine", false, "내 템플릿만 조회")
	kakaoTemplateListCmd.Flags().StringVar(&kakaoTplListFlagStartKey, "start-key", "", "페이지네이션 시작 키")
	kakaoTemplateListCmd.Flags().IntVar(&kakaoTplListFlagLimit, "limit", 20, "조회 건수")
	kakaoTemplateCmd.AddCommand(kakaoTemplateListCmd)
}

func runKakaoTemplateList(cmd *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}

	params := url.Values{}
	params.Set("limit", strconv.Itoa(kakaoTplListFlagLimit))
	if kakaoTplListFlagName != "" {
		params.Set("name", kakaoTplListFlagName)
	}
	if kakaoTplListFlagChannelID != "" {
		params.Set("channelId", kakaoTplListFlagChannelID)
	}
	if kakaoTplListFlagChannelGroupID != "" {
		params.Set("channelGroupId", kakaoTplListFlagChannelGroupID)
	}
	if kakaoTplListFlagTemplateID != "" {
		params.Set("templateId", kakaoTplListFlagTemplateID)
	}
	if kakaoTplListFlagStatus != "" {
		params.Set("status", kakaoTplListFlagStatus)
	}
	if kakaoTplListFlagIsHidden {
		params.Set("isHidden", "true")
	}
	if kakaoTplListFlagIsMine {
		params.Set("isMine", "true")
	}
	if kakaoTplListFlagStartKey != "" {
		params.Set("startKey", kakaoTplListFlagStartKey)
	}

	raw, err := c.Get(ctx(), "kakao/v2/templates", params)
	if err != nil {
		return fmt.Errorf("알림톡 템플릿 목록 조회 실패: %w", err)
	}

	p := printer()
	if flagJSON {
		return p.PrintJSON(raw)
	}

	var resp types.KakaoTemplateListResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("응답 파싱 실패: %w", err)
	}

	headers := []string{"TEMPLATE ID", "NAME", "STATUS", "TYPE", "CHANNEL ID", "CREATED"}
	rows := make([][]string, 0, len(resp.TemplateList))
	for i := range resp.TemplateList {
		tpl := &resp.TemplateList[i]
		rows = append(rows, []string{
			tpl.TemplateID,
			tpl.Name,
			types.DisplayStatus(tpl.Status),
			tpl.DisplayMessageType(),
			tpl.DisplayChannelRef(),
			types.DisplayDate(tpl.DateCreated),
		})
	}
	p.FormatTable(headers, rows)

	if resp.NextKey != "" {
		_, _ = fmt.Fprintf(errOut(), "\n다음 페이지: solactl kakao template list --start-key %s\n", resp.NextKey)
	}

	return nil
}
