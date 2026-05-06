package cmd

import "github.com/spf13/cobra"

var quotaCmd = &cobra.Command{
	Use:   "quota",
	Short: "발송 한도를 조회하고 증가 요청을 제출합니다",
	Long: `발송 한도(quota) 관련 명령 그룹입니다.

  solactl quota get             현재 한도 조회
  solactl quota request         한도 증가 요청 제출
  solactl quota list-requests   제출한 요청 이력 조회`,
}

func init() {
	rootCmd.AddCommand(quotaCmd)
}
