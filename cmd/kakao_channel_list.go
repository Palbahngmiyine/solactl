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
	kakaoChListFlagChannelID    string
	kakaoChListFlagSearchID     string
	kakaoChListFlagPhoneNumber  string
	kakaoChListFlagCategoryCode string
	kakaoChListFlagIsMine       bool
	kakaoChListFlagStartKey     string
	kakaoChListFlagLimit        int
)

var kakaoChannelListCmd = &cobra.Command{
	Use:   "list",
	Short: "카카오톡 채널 목록을 조회합니다",
	Long: `카카오톡 채널 목록을 조회합니다.

사용법:
  solactl kakao channel list
  solactl kakao channel list --limit 5
  solactl kakao channel list --search-id @mychannel
  solactl kakao channel list --start-key <nextKey>`,
	RunE: runKakaoChannelList,
}

func init() {
	kakaoChannelListCmd.Flags().StringVar(&kakaoChListFlagChannelID, "channel-id", "", "채널 ID 필터")
	kakaoChannelListCmd.Flags().StringVar(&kakaoChListFlagSearchID, "search-id", "", "검색용 ID 필터")
	kakaoChannelListCmd.Flags().StringVar(&kakaoChListFlagPhoneNumber, "phone-number", "", "전화번호 필터")
	kakaoChannelListCmd.Flags().StringVar(&kakaoChListFlagCategoryCode, "category-code", "", "카테고리 코드 필터")
	kakaoChannelListCmd.Flags().BoolVar(&kakaoChListFlagIsMine, "is-mine", false, "내 채널만 조회")
	kakaoChannelListCmd.Flags().StringVar(&kakaoChListFlagStartKey, "start-key", "", "페이지네이션 시작 키")
	kakaoChannelListCmd.Flags().IntVar(&kakaoChListFlagLimit, "limit", 20, "조회 건수")
	kakaoChannelCmd.AddCommand(kakaoChannelListCmd)
}

func runKakaoChannelList(cmd *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}

	params := url.Values{}
	params.Set("limit", strconv.Itoa(kakaoChListFlagLimit))
	if kakaoChListFlagChannelID != "" {
		params.Set("channelId", kakaoChListFlagChannelID)
	}
	if kakaoChListFlagSearchID != "" {
		params.Set("searchId", kakaoChListFlagSearchID)
	}
	if kakaoChListFlagPhoneNumber != "" {
		params.Set("phoneNumber", kakaoChListFlagPhoneNumber)
	}
	if kakaoChListFlagCategoryCode != "" {
		params.Set("categoryCode", kakaoChListFlagCategoryCode)
	}
	if kakaoChListFlagIsMine {
		params.Set("isMine", "true")
	}
	if kakaoChListFlagStartKey != "" {
		params.Set("startKey", kakaoChListFlagStartKey)
	}

	raw, err := c.Get(ctx(), "kakao/v2/channels", params)
	if err != nil {
		return fmt.Errorf("카카오톡 채널 목록 조회 실패: %w", err)
	}

	p := printer()
	if flagJSON {
		return p.PrintJSON(raw)
	}

	var resp types.KakaoChannelListResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("응답 파싱 실패: %w", err)
	}

	headers := []string{"CHANNEL ID", "SEARCH ID", "PHONE NUMBER", "BRAND", "CREATED"}
	rows := make([][]string, 0, len(resp.ChannelList))
	for i := range resp.ChannelList {
		ch := &resp.ChannelList[i]
		rows = append(rows, []string{
			ch.ChannelID,
			ch.SearchID,
			ch.PhoneNumber,
			types.DisplayBool(ch.IsBrand),
			types.DisplayDate(ch.DateCreated),
		})
	}
	p.FormatTable(headers, rows)

	if resp.NextKey != "" {
		_, _ = fmt.Fprintf(errOut(), "\n다음 페이지: solactl kakao channel list --start-key %s\n", resp.NextKey)
	}

	return nil
}
