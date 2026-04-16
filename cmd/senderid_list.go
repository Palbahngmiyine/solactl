package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/types"
)

var flagAll bool

var senderidListCmd = &cobra.Command{
	Use:   "list",
	Short: "발신번호 목록을 조회합니다",
	Long: `발신번호 목록을 조회합니다.

기본 동작: 활성화된 발신번호만 조회합니다.
  solactl senderid list

모든 발신번호 조회 (비활성 포함):
  solactl senderid list --all`,
	RunE: runSenderIDList,
}

func init() {
	senderidListCmd.Flags().BoolVar(&flagAll, "all", false, "모든 발신번호 조회 (비활성 포함)")
	senderidCmd.AddCommand(senderidListCmd)
}

func runSenderIDList(cmd *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	p := printer()

	raw, err := c.Get(ctx(), "senderid/v1/numbers", nil)
	if err != nil {
		return fmt.Errorf("발신번호 조회 실패: %w", err)
	}

	if p.JSONMode {
		return p.PrintJSON(raw)
	}

	var info types.SenderIDInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return fmt.Errorf("응답 파싱 실패: %w", err)
	}

	headers := []string{"PHONE NUMBER", "STATUS", "METHOD", "EXPIRE"}
	rows := make([][]string, 0, len(info.SenderIDs))
	for i := range info.SenderIDs {
		s := &info.SenderIDs[i]
		if !flagAll && s.Status != "ACTIVE" {
			continue
		}
		rows = append(rows, []string{
			s.PhoneNumber,
			s.DisplayStatus(),
			s.DisplayMethod(),
			s.DisplayExpireAt(),
		})
	}
	p.FormatTable(headers, rows)
	return nil
}
