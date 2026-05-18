package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/types"
)

var balanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "잔액과 포인트를 조회합니다",
	RunE:  runBalance,
}

func init() {
	rootCmd.AddCommand(balanceCmd)
}

type balanceResponse struct {
	Balance int `json:"balance"`
	Point   int `json:"point"`
}

func runBalance(cmd *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}

	raw, err := c.Get(ctx(), "cash/v1/balance", nil)
	if err != nil {
		return fmt.Errorf("잔액 조회 실패: %w", err)
	}

	p := printer()
	if flagJSON {
		return p.PrintJSON(raw)
	}

	var resp balanceResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("응답 파싱 실패: %w", err)
	}

	p.PrintKeyValue(
		"잔액", formatNumber(resp.Balance)+"원",
		"포인트", formatNumber(resp.Point)+"P",
	)
	return nil
}

func formatNumber(n int) string { return types.FormatThousands(n) }
