package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/crm/spec"
)

var crmConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "CRM CLI 설정/캐시 관리",
}

var crmConfigClearCacheCmd = &cobra.Command{
	Use:   "clear-cache",
	Short: "OpenAPI 스펙 캐시를 모두 삭제합니다",
	Long: `~/.solactl/cache/ 아래의 모든 캐시 파일을 삭제합니다.
다음 'solactl crm <리소스> <액션>' 호출 시 OpenAPI 스펙을 새로 받아옵니다.

캐시 TTL은 1시간입니다. 백엔드에 새 API가 추가되었는데 TTL이 만료되지 않은 경우
이 명령으로 강제 갱신할 수 있습니다.`,
	RunE: runCrmConfigClearCache,
}

func init() {
	crmConfigCmd.AddCommand(crmConfigClearCacheCmd)
	crmCmd.AddCommand(crmConfigCmd)
}

func runCrmConfigClearCache(_ *cobra.Command, _ []string) error {
	if err := spec.ClearCache(); err != nil {
		return fmt.Errorf("캐시 삭제 실패: %w", err)
	}
	dir, _ := spec.CacheDirPath()
	if dir != "" {
		_, _ = fmt.Fprintf(errOut(), "캐시 디렉토리를 비웠습니다: %s\n", dir)
	} else {
		_, _ = fmt.Fprintln(errOut(), "캐시 디렉토리를 비웠습니다.")
	}
	return nil
}
