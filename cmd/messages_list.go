package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
)

var messagesListCmd = &cobra.Command{
	Use:   "list",
	Short: "발송 내역을 조회합니다",
	RunE:  runMessagesList,
}

var (
	msgListFlagLimit     int
	msgListFlagStartDate string
	msgListFlagEndDate   string
	msgListFlagType      string
	msgListFlagStartKey  string
)

func init() {
	messagesListCmd.Flags().IntVar(&msgListFlagLimit, "limit", 20, "조회 건수")
	messagesListCmd.Flags().StringVar(&msgListFlagStartDate, "start-date", "", "시작 날짜 (YYYY-MM-DD)")
	messagesListCmd.Flags().StringVar(&msgListFlagEndDate, "end-date", "", "종료 날짜 (YYYY-MM-DD)")
	messagesListCmd.Flags().StringVar(&msgListFlagType, "type", "", "메시지 타입 (SMS, LMS, MMS, ...)")
	messagesListCmd.Flags().StringVar(&msgListFlagStartKey, "start-key", "", "페이지네이션 시작 키")
	messagesCmd.AddCommand(messagesListCmd)
}

type messageListResponse struct {
	MessageList map[string]messageListItem `json:"messageList"`
	NextKey     string                     `json:"nextKey,omitempty"`
}

type messageListItem struct {
	MessageID   string `json:"messageId"`
	To          string `json:"to"`
	From        string `json:"from"`
	Type        string `json:"type"`
	StatusCode  string `json:"statusCode"`
	DateCreated string `json:"dateCreated"`
}

func runMessagesList(cmd *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}

	params := url.Values{}
	params.Set("limit", strconv.Itoa(msgListFlagLimit))
	if msgListFlagStartDate != "" {
		params.Set("startDate", msgListFlagStartDate)
	}
	if msgListFlagEndDate != "" {
		params.Set("endDate", msgListFlagEndDate)
	}
	if msgListFlagType != "" {
		params.Set("type", msgListFlagType)
	}
	if msgListFlagStartKey != "" {
		params.Set("startKey", msgListFlagStartKey)
	}

	raw, err := c.Get(ctx(), "messages/v4/list", params)
	if err != nil {
		return fmt.Errorf("발송 내역 조회 실패: %w", err)
	}

	p := printer()
	if flagJSON {
		return p.PrintJSON(raw)
	}

	var resp messageListResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("응답 파싱 실패: %w", err)
	}

	headers := []string{"MESSAGE ID", "TO", "TYPE", "STATUS", "DATE"}

	// Collect and sort keys for deterministic output
	ids := make([]string, 0, len(resp.MessageList))
	for id := range resp.MessageList {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var rows [][]string
	for _, id := range ids {
		item := resp.MessageList[id]
		date := item.DateCreated
		if len(date) > 16 {
			date = date[:16]
		}
		rows = append(rows, []string{id, item.To, item.Type, item.StatusCode, date})
	}

	p.FormatTable(headers, rows)

	if resp.NextKey != "" {
		_, _ = fmt.Fprintf(out(), "\n다음 페이지: solactl messages list --start-key %q\n", resp.NextKey)
	}

	return nil
}
