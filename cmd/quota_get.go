package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/types"
)

var quotaGetCmd = &cobra.Command{
	Use:   "get",
	Short: "현재 발송 한도를 조회합니다",
	Long: `현재 계정의 발송 한도를 조회합니다.

응답에는 현재 한도, 최소 / 최대 허용값, 자동 조정 여부, 해외발송 한도가 포함됩니다.`,
	RunE: runQuotaGet,
}

func init() {
	quotaCmd.AddCommand(quotaGetCmd)
}

func runQuotaGet(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}

	raw, err := c.Get(ctx(), "quota/v1/me", nil)
	if err != nil {
		return fmt.Errorf("발송 한도 조회 실패: %w", err)
	}

	p := printer()
	if flagJSON {
		return p.PrintJSON(raw)
	}

	var info types.QuotaInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return fmt.Errorf("응답 파싱 실패: %w", err)
	}

	p.PrintKeyValue(
		"현재 한도", formatNumber(info.Quota),
		"최소값", formatNumber(info.Min),
		"최대값", formatNumber(info.Max),
		"자동 조정", types.DisplayBool(info.AutoAdjustment),
		"해외발송 한도", formatNumber(info.OverseasQuota),
	)
	return nil
}
