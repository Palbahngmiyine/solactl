package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var flagStatus string

var senderidUpdateCmd = &cobra.Command{
	Use:   "update <phoneNumber>",
	Short: "발신번호 상태를 변경합니다",
	Long: `발신번호 상태를 변경합니다.

사용법:
  solactl senderid update 01012345678 --status ACTIVE
  solactl senderid update 01012345678 --status INACTIVE`,
	Args: cobra.ExactArgs(1),
	RunE: runSenderIDUpdate,
}

func init() {
	senderidUpdateCmd.Flags().StringVar(&flagStatus, "status", "", "변경할 상태 (ACTIVE|INACTIVE)")
	_ = senderidUpdateCmd.MarkFlagRequired("status")
	senderidCmd.AddCommand(senderidUpdateCmd)
}

func runSenderIDUpdate(cmd *cobra.Command, args []string) error {
	phoneNumber := args[0]

	c, err := newClient()
	if err != nil {
		return err
	}

	body := map[string]string{"status": flagStatus}
	_, err = c.Put(ctx(), fmt.Sprintf("senderid/v1/numbers/%s", phoneNumber), body)
	if err != nil {
		return fmt.Errorf("발신번호 상태 변경 실패: %w", err)
	}

	_, _ = fmt.Fprintf(out(), "발신번호 %s의 상태가 %s로 변경되었습니다.\n", phoneNumber, flagStatus)
	return nil
}
