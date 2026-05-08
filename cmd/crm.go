package cmd

import (
	"github.com/spf13/cobra"
)

// crmCmd is the parent for all CRM (`crm-core`) operations. The static
// children (`config clear-cache`, `mcp`) are registered eagerly; resource
// trees (`<resource> <action>`) are added dynamically by RegisterDynamicCRM
// at startup, after credentials and the OpenAPI spec have been resolved.
//
// The implementation mirrors @solapi/crm-cli (sdk/cli) — see
// docs/crm-cli-spec.md for the full design rationale.
var crmCmd = &cobra.Command{
	Use:   "crm",
	Short: "SOLAPI CRM 리소스 (entities/records/...) 를 동적 명령으로 노출합니다",
	Long: `SOLAPI CRM (crm-core) API를 CLI에 노출합니다.

사용 가능한 동적 명령은 백엔드 OpenAPI 스펙으로부터 자동 생성됩니다:
  solactl crm <리소스> <액션> [경로인자] [--옵션]

예시:
  solactl crm entities list
  solactl crm records get RECxxx
  solactl crm records create --data '{"entityId":"ENxxx","name":"홍길동"}'
  solactl crm records list --entityId ENxxx --format csv > export.csv

캐시 무효화:
  solactl crm config clear-cache

인증/프로필은 'solactl configure'와 root persistent 플래그(--profile, --api-key 등)를 그대로 사용합니다.`,
}

func init() {
	rootCmd.AddCommand(crmCmd)
}
