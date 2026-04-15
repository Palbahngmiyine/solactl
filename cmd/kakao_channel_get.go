package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/types"
)

var kakaoChannelGetCmd = &cobra.Command{
	Use:   "get <channelId>",
	Short: "카카오톡 채널 상세 정보를 조회합니다",
	Long: `카카오톡 채널의 상세 정보를 조회합니다.

사용법:
  solactl kakao channel get KA01PF240000000000000000000000000`,
	Args: cobra.ExactArgs(1),
	RunE: runKakaoChannelGet,
}

func init() {
	kakaoChannelCmd.AddCommand(kakaoChannelGetCmd)
}

func runKakaoChannelGet(cmd *cobra.Command, args []string) error {
	channelID := args[0]

	c, err := newClient()
	if err != nil {
		return err
	}

	raw, err := c.Get(ctx(), fmt.Sprintf("kakao/v2/channels/%s", channelID), nil)
	if err != nil {
		return fmt.Errorf("카카오톡 채널 조회 실패: %w", err)
	}

	p := printer()
	if flagJSON {
		return p.PrintJSON(raw)
	}

	var ch types.KakaoChannel
	if err := json.Unmarshal(raw, &ch); err != nil {
		return fmt.Errorf("응답 파싱 실패: %w", err)
	}

	p.PrintKeyValue(
		"Channel ID", ch.ChannelID,
		"Search ID", ch.SearchID,
		"Phone Number", ch.PhoneNumber,
		"Channel Name", displayOptionalStr(ch.ChannelName),
		"Brand", types.DisplayBool(ch.IsBrand),
		"Account ID", ch.AccountID,
		"Created", types.DisplayDate(ch.DateCreated),
		"Updated", types.DisplayDate(ch.DateUpdated),
	)

	return nil
}

// displayOptionalStr returns the string or "-" if empty.
func displayOptionalStr(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
