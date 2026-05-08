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
	"github.com/solapi/solactl/pkg/output"
	"github.com/solapi/solactl/pkg/types"
	"github.com/solapi/solactl/pkg/validation"
)

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "메시지를 발송합니다",
}

// Shared flags available to all send subcommands.
var (
	sendFlagTo             string
	sendFlagFrom           string
	sendFlagText           string
	sendFlagScheduled      string
	sendFlagFile           string // CSV file for bulk sending
	sendFlagSkipValidation bool
	sendFlagStrict         bool
)

func init() {
	sendCmd.PersistentFlags().StringVar(&sendFlagTo, "to", "", "수신번호 (쉼표로 구분 가능)")
	sendCmd.PersistentFlags().StringVar(&sendFlagFrom, "from", "", "발신번호")
	sendCmd.PersistentFlags().StringVar(&sendFlagText, "text", "", "메시지 내용")
	sendCmd.PersistentFlags().StringVar(&sendFlagScheduled, "scheduled", "", "예약 발송 시간 (ISO 8601)")
	sendCmd.PersistentFlags().StringVar(&sendFlagFile, "file", "", "수신자 CSV 파일 경로")
	sendCmd.PersistentFlags().BoolVar(&sendFlagSkipValidation, "skip-validation", false, "클라이언트 사이드 검증 건너뛰기")
	sendCmd.PersistentFlags().BoolVar(&sendFlagStrict, "strict", false, "엄격 검증 모드 활성화")

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

// sendMessages validates and sends messages via the API.
// If the message list exceeds maxBatchSize, it auto-splits into multiple API calls.
func sendMessages(c *client.Client, msgs []types.Message) error {
	// Client-side validation (skip with --skip-validation)
	if !sendFlagSkipValidation {
		opts := validation.Options{
			Strict:         sendFlagStrict,
			AutoTypeDetect: true,
		}
		if errs := validation.ValidateMessages(msgs, opts); errs != nil {
			p := &output.Printer{Writer: errOut()}
			_, _ = fmt.Fprintf(errOut(), "검증 오류 %d건:\n", len(errs))
			headers := []string{"번호", "필드", "오류코드", "메시지"}
			var rows [][]string
			for _, e := range errs {
				rows = append(rows, []string{
					fmt.Sprintf("[%d]", e.Index+1),
					e.Field,
					e.Code,
					e.Message,
				})
			}
			p.FormatTable(headers, rows)
			return fmt.Errorf("검증 실패: %d건의 오류가 발견되었습니다", len(errs))
		}
	}

	showList := true
	agent := types.DefaultAgent(version.Version)
	totalBatches := (len(msgs) + maxBatchSize - 1) / maxBatchSize
	batchNum := 0

	for start := 0; start < len(msgs); start += maxBatchSize {
		end := min(start+maxBatchSize, len(msgs))
		batch := msgs[start:end]
		batchNum++

		if totalBatches > 1 {
			_, _ = fmt.Fprintf(errOut(), "[%d/%d] %d건 발송 중...\n", batchNum, totalBatches, len(batch))
		}

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
		_, _ = fmt.Fprintln(out())
		_, _ = fmt.Fprintln(out(), "실패 메시지:")
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
	defer func() { _ = f.Close() }()

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

// resolveFrom returns the --from value if set, otherwise queries the senderid API
// and auto-selects: 1 active → use it, 0 or 2+ → error with guidance.
func resolveFrom(c *client.Client) (string, error) {
	if sendFlagFrom != "" {
		return sendFlagFrom, nil
	}

	raw, err := c.Get(ctx(), "senderid/v1/numbers/active", nil)
	if err != nil {
		return "", fmt.Errorf("발신번호(--from)를 입력하세요 (발신번호 조회 실패: %w)", err)
	}

	var phones []string
	if err := json.Unmarshal(raw, &phones); err != nil {
		return "", fmt.Errorf("발신번호 자동 선택 실패 (응답 파싱 오류: %w). --from으로 직접 지정하세요", err)
	}

	switch len(phones) {
	case 0:
		return "", fmt.Errorf("등록된 활성 발신번호가 없습니다. solactl senderid list로 확인하세요")
	case 1:
		_, _ = fmt.Fprintf(errOut(), "발신번호 자동 선택: %s\n", phones[0])
		return phones[0], nil
	default:
		var lines []string
		for _, phone := range phones {
			lines = append(lines, "  "+phone)
		}
		return "", fmt.Errorf("활성 발신번호가 %d개입니다. --from으로 지정하세요:\n%s",
			len(phones), strings.Join(lines, "\n"))
	}
}
