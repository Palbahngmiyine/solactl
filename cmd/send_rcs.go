package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/types"
)

var sendRCSCmd = &cobra.Command{
	Use:   "rcs",
	Short: "RCS 메시지를 발송합니다",
	RunE:  runSendRCS,
}

var (
	sendRCSFlagBrandID     string
	sendRCSFlagTemplateID  string
	sendRCSFlagVariables   string
	sendRCSFlagSubject     string
	sendRCSFlagImage       string
	sendRCSFlagMmsType     string
	sendRCSFlagCopyAllowed bool
)

func init() {
	sendRCSCmd.Flags().StringVar(&sendRCSFlagBrandID, "brand-id", "", "RCS 브랜드 ID (필수)")
	sendRCSCmd.Flags().StringVar(&sendRCSFlagTemplateID, "template-id", "", "RCS 템플릿 ID")
	sendRCSCmd.Flags().StringVar(&sendRCSFlagVariables, "variables", "", "변수 (JSON 문자열)")
	sendRCSCmd.Flags().StringVar(&sendRCSFlagSubject, "subject", "", "메시지 제목")
	sendRCSCmd.Flags().StringVar(&sendRCSFlagImage, "image", "", "이미지 파일 경로")
	sendRCSCmd.Flags().StringVar(&sendRCSFlagMmsType, "mms-type", "", "RCS MMS 타입")
	sendRCSCmd.Flags().BoolVar(&sendRCSFlagCopyAllowed, "copy-allowed", false, "복사 허용")
	sendCmd.AddCommand(sendRCSCmd)
}

func runSendRCS(cmd *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}

	if sendFlagTo == "" && sendFlagFile == "" {
		return fmt.Errorf("수신번호(--to)를 입력하세요")
	}
	if sendRCSFlagBrandID == "" {
		return fmt.Errorf("RCS 브랜드 ID(--brand-id)를 입력하세요")
	}
	if sendFlagText == "" && sendRCSFlagTemplateID == "" {
		return fmt.Errorf("메시지 내용(--text)을 입력하세요 (또는 --template-id 사용)")
	}

	from, err := resolveFrom(c)
	if err != nil {
		return err
	}

	variables, err := parseVariables(sendRCSFlagVariables)
	if err != nil {
		return err
	}

	rcsOpts := &types.RCSOptions{
		BrandID:   sendRCSFlagBrandID,
		Variables: variables,
		MmsType:   sendRCSFlagMmsType,
	}
	if sendRCSFlagTemplateID != "" {
		rcsOpts.TemplateID = sendRCSFlagTemplateID
	}
	if sendRCSFlagCopyAllowed {
		rcsOpts.CopyAllowed = new(true)
	}

	// Handle image upload for RCS
	var imageID string
	if sendRCSFlagImage != "" {
		imageID, err = uploadImage(c, sendRCSFlagImage, "RCS")
		if err != nil {
			return err
		}
	}

	var msgs []types.Message

	if sendFlagFile != "" {
		msgs, err = loadCSVMessages(sendFlagFile, from, sendFlagText)
		if err != nil {
			return err
		}
		for i := range msgs {
			msgs[i].RCSOptions = rcsOpts
			msgs[i].Subject = sendRCSFlagSubject
			if imageID != "" {
				msgs[i].ImageID = imageID
			}
		}
	} else {
		msgs, err = buildMessagesFromFlags(func(to string) types.Message {
			return types.Message{
				To:         to,
				From:       from,
				Text:       sendFlagText,
				Subject:    sendRCSFlagSubject,
				ImageID:    imageID,
				RCSOptions: rcsOpts,
			}
		})
		if err != nil {
			return err
		}
	}

	return sendMessages(c, msgs)
}
