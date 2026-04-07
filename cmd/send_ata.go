package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/types"
)

var sendATACmd = &cobra.Command{
	Use:   "ata",
	Short: "카카오 알림톡을 발송합니다",
	RunE:  runSendATA,
}

var (
	sendATAFlagPfID       string
	sendATAFlagTemplateID string
	sendATAFlagVariables  string
	sendATAFlagTitle      string
	sendATAFlagDisableSms bool
	sendATAFlagButtons    string
)

func init() {
	sendATACmd.Flags().StringVar(&sendATAFlagPfID, "pfid", "", "카카오 채널 ID (필수)")
	sendATACmd.Flags().StringVar(&sendATAFlagTemplateID, "template-id", "", "알림톡 템플릿 ID (필수)")
	sendATACmd.Flags().StringVar(&sendATAFlagVariables, "variables", "", "변수 (JSON 문자열)")
	sendATACmd.Flags().StringVar(&sendATAFlagTitle, "title", "", "강조 표기 제목")
	sendATACmd.Flags().BoolVar(&sendATAFlagDisableSms, "disable-sms", false, "SMS 대체 발송 비활성화")
	sendATACmd.Flags().StringVar(&sendATAFlagButtons, "buttons", "", "버튼 목록 (JSON 배열)")
	sendCmd.AddCommand(sendATACmd)
}

func runSendATA(cmd *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}

	if sendFlagTo == "" && sendFlagFile == "" {
		return fmt.Errorf("수신번호(--to)를 입력하세요")
	}
	if sendATAFlagPfID == "" {
		return fmt.Errorf("카카오 채널 ID(--pfid)를 입력하세요")
	}
	if sendATAFlagTemplateID == "" {
		return fmt.Errorf("알림톡 템플릿 ID(--template-id)를 입력하세요")
	}
	if sendFlagText == "" {
		return fmt.Errorf("메시지 내용(--text)을 입력하세요")
	}

	variables, err := parseVariables(sendATAFlagVariables)
	if err != nil {
		return err
	}

	buttons, err := parseKakaoButtons(sendATAFlagButtons)
	if err != nil {
		return err
	}
	if len(buttons) > 5 {
		return fmt.Errorf("버튼은 최대 5개까지 가능합니다")
	}

	kakaoOpts := &types.KakaoOptions{
		PfID:       sendATAFlagPfID,
		TemplateID: sendATAFlagTemplateID,
		Variables:  variables,
		Buttons:    buttons,
		Title:      sendATAFlagTitle,
	}
	if sendATAFlagDisableSms {
		kakaoOpts.DisableSms = boolPtr(true)
	}

	var msgs []types.Message

	if sendFlagFile != "" {
		msgs, err = loadCSVMessages(sendFlagFile, sendFlagFrom, sendFlagText)
		if err != nil {
			return err
		}
		for i := range msgs {
			msgs[i].KakaoOptions = kakaoOpts
		}
	} else {
		msgs, err = buildMessagesFromFlags(func(to string) types.Message {
			return types.Message{
				To:           to,
				From:         sendFlagFrom,
				Text:         sendFlagText,
				KakaoOptions: kakaoOpts,
			}
		})
		if err != nil {
			return err
		}
	}

	return sendMessages(c, msgs)
}

// parseVariables parses a JSON string into a map[string]string for variable substitution.
// Returns nil map and no error for empty input.
func parseVariables(s string) (map[string]string, error) {
	if s == "" {
		return nil, nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil, fmt.Errorf("변수 JSON 파싱 실패: %w", err)
	}

	result := make(map[string]string, len(raw))
	for k, v := range raw {
		var str string
		if err := json.Unmarshal(v, &str); err != nil {
			return nil, fmt.Errorf("변수 값은 문자열이어야 합니다 (키: %s): %w", k, err)
		}
		result[k] = str
	}
	return result, nil
}

// parseKakaoButtons parses a JSON array string into a slice of KakaoButton.
// Returns nil slice and no error for empty input.
func parseKakaoButtons(s string) ([]types.KakaoButton, error) {
	if s == "" {
		return nil, nil
	}

	var buttons []types.KakaoButton
	if err := json.Unmarshal([]byte(s), &buttons); err != nil {
		return nil, fmt.Errorf("버튼 JSON 파싱 실패: %w", err)
	}
	return buttons, nil
}
