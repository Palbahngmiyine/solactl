package cmd

import (
	"github.com/spf13/cobra"
)

// crmCmd is the parent for all CRM (`crm-core`) operations. The static
// children (`config clear-cache`, `mcp`) are registered eagerly; resource
// trees (`<resource> <action>`) are added dynamically by RegisterDynamicCRM
// at startup, after the OpenAPI spec has been resolved.
//
// The implementation mirrors @solapi/crm-cli (sdk/cli) — see
// docs/crm-cli-spec.md for the full design rationale.
var crmCmd = &cobra.Command{
	Use:   "crm",
	Short: "SOLAPI CRM의 고객 데이터와 레코드를 조회하고 관리합니다",
	Long: `SOLAPI CRM에서 사용하는 고객 데이터, 엔티티, 레코드를 CLI로 조회하고 관리합니다.

사용 가능한 작업은 로그인한 계정의 CRM 기능에 맞춰 제공됩니다.
아래 형식으로 리소스와 작업을 선택하세요:
  solactl crm <리소스> <작업> [값] [옵션]

예시:
  solactl crm entities list
  solactl crm records get RECxxx
  solactl crm records create --data '{"entityId":"ENxxx","name":"홍길동"}'
  solactl crm records list --entityId ENxxx --format csv > export.csv

최신 CRM 명령이 보이지 않을 때:
  solactl crm config clear-cache

인증 정보는 기존 solactl 설정을 사용합니다. 다른 프로필로 실행하려면 --profile을 함께 지정하세요.`,
}

func init() {
	rootCmd.AddCommand(crmCmd)
}
