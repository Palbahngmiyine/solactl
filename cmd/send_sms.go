package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/types"
)

var sendSMSCmd = &cobra.Command{
	Use:   "sms",
	Short: "SMS 메시지를 발송합니다",
	RunE:  runSendSMS,
}

func init() {
	sendCmd.AddCommand(sendSMSCmd)
}

func runSendSMS(cmd *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}

	var msgs []types.Message

	if sendFlagFile != "" {
		if sendFlagFrom == "" {
			return fmt.Errorf("발신번호(--from)를 입력하세요")
		}
		msgs, err = loadCSVMessages(sendFlagFile, sendFlagFrom, sendFlagText)
		if err != nil {
			return err
		}
	} else {
		if sendFlagTo == "" {
			return fmt.Errorf("수신번호(--to)를 입력하세요")
		}
		if sendFlagText == "" {
			return fmt.Errorf("메시지 내용(--text)을 입력하세요")
		}
		if sendFlagFrom == "" {
			return fmt.Errorf("발신번호(--from)를 입력하세요")
		}

		msgs, err = buildMessagesFromFlags(func(to string) types.Message {
			return types.Message{
				To:   to,
				From: sendFlagFrom,
				Text: sendFlagText,
			}
		})
		if err != nil {
			return err
		}
	}

	return sendMessages(c, msgs)
}
