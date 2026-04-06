package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var flagYes bool

var senderidDeleteCmd = &cobra.Command{
	Use:   "delete <phoneNumber>",
	Short: "발신번호를 삭제합니다",
	Long: `발신번호를 삭제합니다.

사용법:
  solactl senderid delete 01012345678
  solactl senderid delete 01012345678 --yes  # 확인 없이 삭제`,
	Args: cobra.ExactArgs(1),
	RunE: runSenderIDDelete,
}

// stdinReader is used for reading user confirmation. Tests can override this.
var stdinReader *bufio.Reader

func init() {
	senderidDeleteCmd.Flags().BoolVarP(&flagYes, "yes", "y", false, "확인 없이 삭제")
	senderidCmd.AddCommand(senderidDeleteCmd)
}

func runSenderIDDelete(cmd *cobra.Command, args []string) error {
	phoneNumber := args[0]

	if !flagYes {
		_, _ = fmt.Fprintf(out(), "\u26a0 발신번호 %s을 삭제하시겠습니까? (y/N): ", phoneNumber)

		reader := stdinReader
		if reader == nil {
			reader = bufio.NewReader(os.Stdin)
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("입력을 읽을 수 없습니다: %w", err)
		}
		answer := strings.TrimSpace(strings.ToLower(line))
		if answer != "y" && answer != "yes" {
			_, _ = fmt.Fprintln(out(), "삭제가 취소되었습니다.")
			return nil
		}
	}

	c, err := newClient()
	if err != nil {
		return err
	}

	_, err = c.Delete(ctx(), fmt.Sprintf("senderid/v1/numbers/%s", phoneNumber))
	if err != nil {
		return fmt.Errorf("발신번호 삭제 실패: %w", err)
	}

	_, _ = fmt.Fprintf(out(), "발신번호 %s이 삭제되었습니다.\n", phoneNumber)
	return nil
}
