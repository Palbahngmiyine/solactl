package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
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

// formatNumber formats an integer with thousand separators.
func formatNumber(n int) string {
	negative := n < 0
	if negative {
		n = -n
	}

	s := strconv.Itoa(n)
	if len(s) <= 3 {
		if negative {
			return "-" + s
		}
		return s
	}

	var b strings.Builder
	offset := len(s) % 3
	if offset > 0 {
		b.WriteString(s[:offset])
	}
	for i := offset; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}

	if negative {
		return "-" + b.String()
	}
	return b.String()
}
