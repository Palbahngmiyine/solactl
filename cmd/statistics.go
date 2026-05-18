package cmd

import "github.com/spf13/cobra"

var statisticsCmd = &cobra.Command{
	Use:     "statistics",
	Aliases: []string{"stats"},
	Short:   "발송 통계를 조회하고 export합니다",
	Long: `발송 통계 관련 명령 그룹입니다.

  solactl statistics export-daily   일별 통계 내역을 CSV/JSON/JSONL로 export`,
}

func init() {
	rootCmd.AddCommand(statisticsCmd)
}
