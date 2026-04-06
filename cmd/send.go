package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/internal/version"
	"github.com/solapi/solactl/pkg/client"
	"github.com/solapi/solactl/pkg/types"
)

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "메시지를 발송합니다",
}

// Shared flags available to all send subcommands.
var (
	sendFlagTo        string
	sendFlagFrom      string
	sendFlagText      string
	sendFlagScheduled string
	sendFlagFile      string // CSV file for bulk sending
)

func init() {
	sendCmd.PersistentFlags().StringVar(&sendFlagTo, "to", "", "수신번호 (쉼표로 구분 가능)")
	sendCmd.PersistentFlags().StringVar(&sendFlagFrom, "from", "", "발신번호")
	sendCmd.PersistentFlags().StringVar(&sendFlagText, "text", "", "메시지 내용")
	sendCmd.PersistentFlags().StringVar(&sendFlagScheduled, "scheduled", "", "예약 발송 시간 (ISO 8601)")
	sendCmd.PersistentFlags().StringVar(&sendFlagFile, "file", "", "수신자 CSV 파일 경로")

	rootCmd.AddCommand(sendCmd)
}

// maxBatchSize is the maximum number of messages per API call.
// Declared as var (not const) so tests can override it to verify batching logic.
var maxBatchSize = 10000

// parseRecipients splits a comma-separated --to value into individual phone numbers,
// trimming whitespace from each.
func parseRecipients(to string) []string {
	parts := strings.Split(to, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// sendMessages builds a SendRequest with the agent and posts to send-many/detail.
// If the message list exceeds maxBatchSize, it auto-splits into multiple API calls.
func sendMessages(c *client.Client, msgs []types.Message) error {
	showList := true
	agent := types.DefaultAgent(version.Version)

	for start := 0; start < len(msgs); start += maxBatchSize {
		end := start + maxBatchSize
		if end > len(msgs) {
			end = len(msgs)
		}
		batch := msgs[start:end]

		req := types.SendRequest{
			Messages:        batch,
			Agent:           agent,
			ShowMessageList: &showList,
		}
		if sendFlagScheduled != "" {
			req.ScheduledDate = sendFlagScheduled
		}

		data, err := c.Post(ctx(), "messages/v4/send-many/detail", req)
		if err != nil {
			return fmt.Errorf("발송 실패: %w", err)
		}

		if err := printSendResult(data); err != nil {
			return err
		}
	}
	return nil
}

// printSendResult formats and prints the send-many/detail response.
func printSendResult(data json.RawMessage) error {
	p := printer()

	if flagJSON {
		return p.PrintJSON(data)
	}

	var resp types.SendResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("응답 파싱 실패: %w", err)
	}

	p.PrintKeyValue(
		"Group ID", resp.GroupInfo.GroupID,
		"상태", resp.GroupInfo.Status,
		"총 건수", fmt.Sprintf("%d", resp.GroupInfo.Count.Total),
		"등록 성공", fmt.Sprintf("%d", resp.GroupInfo.Count.RegisteredSuccess),
		"등록 실패", fmt.Sprintf("%d", resp.GroupInfo.Count.RegisteredFailed),
	)

	if len(resp.FailedMessageList) > 0 {
		fmt.Fprintln(out())
		fmt.Fprintln(out(), "실패 메시지:")
		headers := []string{"수신번호", "상태코드", "상태메시지"}
		var rows [][]string
		for _, f := range resp.FailedMessageList {
			rows = append(rows, []string{f.To, f.StatusCode, f.StatusMessage})
		}
		p.FormatTable(headers, rows)
	}

	return nil
}

// loadCSVMessages reads a CSV file, uses the "to" column for recipients,
// and substitutes {{column_name}} placeholders in textTemplate.
func loadCSVMessages(filepath, from, textTemplate string) ([]types.Message, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("CSV 파일 열기 실패: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV 읽기 실패: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV 파일에 데이터가 없습니다 (헤더만 존재)")
	}

	headers := records[0]
	toIdx := -1
	for i, h := range headers {
		if strings.TrimSpace(strings.ToLower(h)) == "to" {
			toIdx = i
			break
		}
	}
	if toIdx < 0 {
		return nil, fmt.Errorf("CSV 헤더에 'to' 컬럼이 필요합니다")
	}

	var msgs []types.Message
	for _, row := range records[1:] {
		if toIdx >= len(row) {
			continue
		}
		to := strings.TrimSpace(row[toIdx])
		if to == "" {
			continue
		}

		text := textTemplate
		for i, h := range headers {
			if i < len(row) {
				text = strings.ReplaceAll(text, "{{"+h+"}}", row[i])
			}
		}

		msgs = append(msgs, types.Message{
			To:   to,
			From: from,
			Text: text,
		})
	}

	if len(msgs) == 0 {
		return nil, fmt.Errorf("CSV에서 유효한 수신번호를 찾을 수 없습니다")
	}

	return msgs, nil
}

// buildMessagesFromFlags creates messages from --to and --from flags.
// msgBuilder is called for each recipient to build the Message.
func buildMessagesFromFlags(msgBuilder func(to string) types.Message) ([]types.Message, error) {
	recipients := parseRecipients(sendFlagTo)
	if len(recipients) == 0 {
		return nil, fmt.Errorf("수신번호(--to)를 입력하세요")
	}

	var msgs []types.Message
	for _, to := range recipients {
		msgs = append(msgs, msgBuilder(to))
	}
	return msgs, nil
}
