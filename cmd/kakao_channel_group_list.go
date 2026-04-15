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
	kakaoChgListFlagGroupID  string
	kakaoChgListFlagName     string
	kakaoChgListFlagStatus   string
	kakaoChgListFlagIsMine   bool
	kakaoChgListFlagStartKey string
	kakaoChgListFlagLimit    int
)

var kakaoChannelGroupListCmd = &cobra.Command{
	Use:   "list",
	Short: "카카오톡 채널 그룹 목록을 조회합니다",
	Long: `카카오톡 채널 그룹 목록을 조회합니다.

사용법:
  solactl kakao channel-group list
  solactl kakao channel-group list --limit 5
  solactl kakao channel-group list --status APPROVED
  solactl kakao channel-group list --start-key <nextKey>`,
	RunE: runKakaoChannelGroupList,
}

func init() {
	kakaoChannelGroupListCmd.Flags().StringVar(&kakaoChgListFlagGroupID, "channel-group-id", "", "채널 그룹 ID 필터")
	kakaoChannelGroupListCmd.Flags().StringVar(&kakaoChgListFlagName, "name", "", "그룹 이름 필터")
	kakaoChannelGroupListCmd.Flags().StringVar(&kakaoChgListFlagStatus, "status", "", "상태 필터 (PENDING/INSPECTING/APPROVED/REJECTED)")
	kakaoChannelGroupListCmd.Flags().BoolVar(&kakaoChgListFlagIsMine, "is-mine", false, "내 그룹만 조회")
	kakaoChannelGroupListCmd.Flags().StringVar(&kakaoChgListFlagStartKey, "start-key", "", "페이지네이션 시작 키")
	kakaoChannelGroupListCmd.Flags().IntVar(&kakaoChgListFlagLimit, "limit", 20, "조회 건수")
	kakaoChannelGroupCmd.AddCommand(kakaoChannelGroupListCmd)
}

func runKakaoChannelGroupList(cmd *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}

	params := url.Values{}
	params.Set("limit", strconv.Itoa(kakaoChgListFlagLimit))
	if kakaoChgListFlagGroupID != "" {
		params.Set("channelGroupId", kakaoChgListFlagGroupID)
	}
	if kakaoChgListFlagName != "" {
		params.Set("name", kakaoChgListFlagName)
	}
	if kakaoChgListFlagStatus != "" {
		params.Set("status", kakaoChgListFlagStatus)
	}
	if kakaoChgListFlagIsMine {
		params.Set("isMine", "true")
	}
	if kakaoChgListFlagStartKey != "" {
		params.Set("startKey", kakaoChgListFlagStartKey)
	}

	raw, err := c.Get(ctx(), "kakao/v2/channel-groups", params)
	if err != nil {
		return fmt.Errorf("카카오톡 채널 그룹 목록 조회 실패: %w", err)
	}

	p := printer()
	if flagJSON {
		return p.PrintJSON(raw)
	}

	var resp types.KakaoChannelGroupListResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("응답 파싱 실패: %w", err)
	}

	headers := []string{"GROUP ID", "NAME", "STATUS", "TYPE", "CHANNELS", "BRAND", "CREATED"}
	rows := make([][]string, 0, len(resp.ChannelGroupList))
	for i := range resp.ChannelGroupList {
		g := &resp.ChannelGroupList[i]
		rows = append(rows, []string{
			g.ChannelGroupID,
			g.Name,
			types.DisplayStatus(g.Status),
			types.DisplayStatus(g.Type),
			g.DisplayChannelCount(),
			types.DisplayBool(g.IsBrand),
			types.DisplayDate(g.DateCreated),
		})
	}
	p.FormatTable(headers, rows)

	if resp.NextKey != "" {
		_, _ = fmt.Fprintf(errOut(), "\n다음 페이지: solactl kakao channel-group list --start-key %s\n", resp.NextKey)
	}

	return nil
}
