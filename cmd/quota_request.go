package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/types"
)

const (
	quotaTargetMin = 50
	quotaTargetMax = 10_000_000
	quotaReasonMax = 500
)

var (
	quotaRequestFlagTarget int
	quotaRequestFlagReason string
)

var quotaRequestCmd = &cobra.Command{
	Use:   "request",
	Short: "발송 한도 증가를 요청합니다",
	Long: `발송 한도 증가를 요청합니다.

요청은 SOLAPI 운영팀의 검토를 거쳐 승인 또는 반려됩니다.
승인 가능성을 높이려면 요청 사유(--reason)에 다음 정보를 가능한 한 구체적으로 포함하세요.

  1) 누구에게 보내는지   — 수신자 그룹의 성격, 동의(수신 동의) 확보 경로
  2) 무엇을 보내는지     — 실제로 발송할 메시지 본문 전문 또는 핵심 예시
                           (검토자가 컨텍스트를 빠르게 파악할 수 있어 승인이 가장 빨라집니다)
  3) 언제 / 얼마나 자주  — 캠페인 일자, 1일 / 1회 발송 예상 건수
  4) 왜 한도가 더 필요한지 — 비즈니스 사유 (이벤트, 정기 알림 등)

사용법:
  solactl quota request --target 5000 --reason "$(cat <<EOF
  대상: 자사몰 회원 4,800명 (가입 시 마케팅 수신 동의 보유)
  내용: '[OO몰] 5월 단독 세일 안내. 회원 한정 30% 쿠폰: <링크>'
  발송 시점: 2026-05-15 14:00 일회성, 약 4,800건
  사유: 정기 캠페인 발송으로 일일 한도 초과 예상
  EOF
  )"

요청 후 처리 흐름:
  - 응답으로 받은 요청 ID(handleKey)와 PENDING 상태가 표시됩니다
  - 검토 결과는 'solactl quota list-requests' 로 추적할 수 있습니다
  - 동일 계정에 PENDING 요청이 이미 있는 경우, 새 요청을 보내면 이전 요청은 자동으로 REJECTED 처리됩니다

제약:
  - --target: 50 이상 10,000,000 이하 (필수)
  - --reason: 최대 500자 (필수)`,
	RunE: runQuotaRequest,
}

func init() {
	quotaRequestCmd.Flags().IntVar(&quotaRequestFlagTarget, "target", 0, "요청할 새 한도값 (50 이상 10,000,000 이하, 필수)")
	quotaRequestCmd.Flags().StringVar(&quotaRequestFlagReason, "reason", "", "요청 사유 (최대 500자, 필수)")
	quotaCmd.AddCommand(quotaRequestCmd)
}

// validateQuotaRequest validates --target / --reason flag values.
// Extracted so it can be exercised by fuzz / unit tests independently of
// the cobra wiring.
func validateQuotaRequest(target int, reason string) error {
	if target == 0 {
		return fmt.Errorf("요청 한도(--target)를 입력하세요")
	}
	if target < quotaTargetMin || target > quotaTargetMax {
		return fmt.Errorf("요청 한도는 %s 이상 %s 이하여야 합니다", formatNumber(quotaTargetMin), formatNumber(quotaTargetMax))
	}
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return fmt.Errorf("요청 사유(--reason)를 입력하세요")
	}
	if utf8.RuneCountInString(reason) > quotaReasonMax {
		return fmt.Errorf("요청 사유는 %d자 이하여야 합니다", quotaReasonMax)
	}
	return nil
}

func runQuotaRequest(_ *cobra.Command, _ []string) error {
	if err := validateQuotaRequest(quotaRequestFlagTarget, quotaRequestFlagReason); err != nil {
		return err
	}

	c, err := newClient()
	if err != nil {
		return err
	}

	body := map[string]any{
		"quota":           quotaRequestFlagTarget,
		"reasonRequested": quotaRequestFlagReason,
	}

	raw, err := c.Post(ctx(), "quota/v1/me/system", body)
	if err != nil {
		return fmt.Errorf("발송 한도 증가 요청 실패: %w", err)
	}

	p := printer()
	if flagJSON {
		return p.PrintJSON(raw)
	}

	var resp types.IncreaseQuotaRequest
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("응답 파싱 실패: %w", err)
	}

	_, _ = fmt.Fprintln(out(), "발송 한도 증가 요청이 접수되었습니다.")
	p.PrintKeyValue(
		"요청 ID", types.DisplayStatus(resp.HandleKey),
		"요청 한도", formatNumber(resp.RequestedQuota),
		"상태", types.DisplayStatus(resp.Status),
	)
	return nil
}
