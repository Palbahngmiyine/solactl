package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/types"
	"github.com/solapi/solactl/pkg/validation"
)

const quotaReasonColumnMax = 30

var (
	quotaListRequestsFlagStatus   string
	quotaListRequestsFlagStartKey string
	quotaListRequestsFlagLimit    int
)

var quotaListRequestsCmd = &cobra.Command{
	Use:   "list-requests",
	Short: "제출한 발송 한도 증가 요청 목록을 조회합니다",
	Long: `제출한 발송 한도 증가 요청 목록을 조회합니다.

상태값:
  PENDING    검토 대기 중
  APPROVED   승인됨 — 한도 적용
  REJECTED   반려됨 (또는 새 요청 제출 시 이전 요청이 자동 반려)

다음 페이지를 보려면 응답 안내에 표시되는 'solactl quota list-requests --start-key <키>' 를 사용하세요.`,
	RunE: runQuotaListRequests,
}

func init() {
	quotaListRequestsCmd.Flags().StringVar(&quotaListRequestsFlagStatus, "status", "", fmt.Sprintf("상태 필터 (%s/%s/%s)", types.QuotaStatusPending, types.QuotaStatusApproved, types.QuotaStatusRejected))
	quotaListRequestsCmd.Flags().StringVar(&quotaListRequestsFlagStartKey, "start-key", "", "페이지네이션 시작 키")
	quotaListRequestsCmd.Flags().IntVar(&quotaListRequestsFlagLimit, "limit", 20, "조회 건수")
	quotaCmd.AddCommand(quotaListRequestsCmd)
}

func runQuotaListRequests(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}

	params := url.Values{}
	params.Set("limit", strconv.Itoa(quotaListRequestsFlagLimit))
	if quotaListRequestsFlagStatus != "" {
		params.Set("status", quotaListRequestsFlagStatus)
	}
	if quotaListRequestsFlagStartKey != "" {
		params.Set("startKey", quotaListRequestsFlagStartKey)
	}

	raw, err := c.Get(ctx(), "quota/v1/me/system", params)
	if err != nil {
		return fmt.Errorf("발송 한도 요청 목록 조회 실패: %w", err)
	}

	p := printer()
	if flagJSON {
		return p.PrintJSON(raw)
	}

	var resp types.IncreaseQuotaListResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("응답 파싱 실패: %w", err)
	}

	headers := []string{"REQUEST ID", "STATUS", "REQUESTED", "REASON", "CREATED"}
	rows := make([][]string, 0, len(resp.IncreaseQuotaList))
	for i := range resp.IncreaseQuotaList {
		r := &resp.IncreaseQuotaList[i]
		rows = append(rows, []string{
			r.HandleKey,
			types.DisplayStatus(r.Status),
			formatNumber(r.RequestedQuota),
			truncateReason(r.ReasonRequested, quotaReasonColumnMax),
			types.DisplayDate(r.DateCreated),
		})
	}
	p.FormatTable(headers, rows)

	if resp.NextKey != "" {
		_, _ = fmt.Fprintf(errOut(), "\n다음 페이지: solactl quota list-requests --start-key %s\n", resp.NextKey)
	}

	return nil
}

// Empty input returns "-" so the table column is not blank.
// Internal whitespace runs (newlines, tabs, repeated spaces) are collapsed
// to a single space — users are encouraged to write multi-line heredoc
// reasons, which would otherwise break FormatTable's per-row layout.
func truncateReason(s string, maxRunes int) string {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return "-"
	}
	if validation.GetRealTextLength(s) <= maxRunes {
		return s
	}
	if maxRunes <= 1 {
		return "…"
	}
	return string([]rune(s)[:maxRunes-1]) + "…"
}
