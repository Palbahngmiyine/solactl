package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var kakaoTplDeleteFlagYes bool

var kakaoTemplateDeleteCmd = &cobra.Command{
	Use:   "delete <templateId>",
	Short: "알림톡 템플릿을 삭제합니다",
	Long: `알림톡 템플릿을 삭제합니다.

사용법:
  solactl kakao template delete KA01TP240101000000000000000000000
  solactl kakao template delete KA01TP240101000000000000000000000 --yes`,
	Args: cobra.ExactArgs(1),
	RunE: runKakaoTemplateDelete,
}

func init() {
	kakaoTemplateDeleteCmd.Flags().BoolVarP(&kakaoTplDeleteFlagYes, "yes", "y", false, "확인 없이 삭제")
	kakaoTemplateCmd.AddCommand(kakaoTemplateDeleteCmd)
}

func runKakaoTemplateDelete(cmd *cobra.Command, args []string) error {
	templateID := args[0]

	if !kakaoTplDeleteFlagYes {
		_, _ = fmt.Fprintf(out(), "\u26a0 알림톡 템플릿 %s을 삭제하시겠습니까? (y/N): ", templateID)

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

	_, err = c.Delete(ctx(), fmt.Sprintf("kakao/v2/templates/%s", templateID))
	if err != nil {
		return fmt.Errorf("알림톡 템플릿 삭제 실패: %w", err)
	}

	_, _ = fmt.Fprintf(out(), "알림톡 템플릿 %s이 삭제되었습니다.\n", templateID)
	return nil
}
