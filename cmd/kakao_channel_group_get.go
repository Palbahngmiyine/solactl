package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/types"
)

var kakaoChannelGroupGetCmd = &cobra.Command{
	Use:   "get <channelGroupId>",
	Short: "카카오톡 채널 그룹 상세 정보를 조회합니다",
	Long: `카카오톡 채널 그룹의 상세 정보와 소속 채널 목록을 조회합니다.

사용법:
  solactl kakao channel-group get KA01GI240000000000000000000000000`,
	Args: cobra.ExactArgs(1),
	RunE: runKakaoChannelGroupGet,
}

func init() {
	kakaoChannelGroupCmd.AddCommand(kakaoChannelGroupGetCmd)
}

func runKakaoChannelGroupGet(cmd *cobra.Command, args []string) error {
	groupID := args[0]

	c, err := newClient()
	if err != nil {
		return err
	}

	raw, err := c.Get(ctx(), fmt.Sprintf("kakao/v2/channel-groups/%s", groupID), nil)
	if err != nil {
		return fmt.Errorf("카카오톡 채널 그룹 조회 실패: %w", err)
	}

	p := printer()
	if flagJSON {
		return p.PrintJSON(raw)
	}

	var group types.KakaoChannelGroup
	if err := json.Unmarshal(raw, &group); err != nil {
		return fmt.Errorf("응답 파싱 실패: %w", err)
	}

	p.PrintKeyValue(
		"Group ID", group.ChannelGroupID,
		"Name", group.Name,
		"Status", types.DisplayStatus(group.Status),
		"Type", types.DisplayStatus(group.Type),
		"Brand", types.DisplayBool(group.IsBrand),
		"Account ID", group.AccountID,
		"Created", types.DisplayDate(group.DateCreated),
		"Updated", types.DisplayDate(group.DateUpdated),
	)

	if len(group.Channels) > 0 {
		_, _ = fmt.Fprintln(out(), "\n소속 채널:")
		headers := []string{"CHANNEL ID", "SEARCH ID", "BRAND"}
		rows := make([][]string, 0, len(group.Channels))
		for i := range group.Channels {
			ch := &group.Channels[i]
			rows = append(rows, []string{
				ch.ChannelID,
				ch.SearchID,
				types.DisplayBool(ch.IsBrand),
			})
		}
		p.FormatTable(headers, rows)
	}

	return nil
}
