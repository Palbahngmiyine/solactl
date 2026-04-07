package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/client"
	"github.com/solapi/solactl/pkg/types"
)

var sendMMSCmd = &cobra.Command{
	Use:   "mms",
	Short: "MMS 메시지를 발송합니다",
	RunE:  runSendMMS,
}

var sendMMSFlagImage   string
var sendMMSFlagSubject string

func init() {
	sendMMSCmd.Flags().StringVar(&sendMMSFlagImage, "image", "", "이미지 파일 경로 (필수)")
	sendMMSCmd.Flags().StringVar(&sendMMSFlagSubject, "subject", "", "메시지 제목")
	sendCmd.AddCommand(sendMMSCmd)
}

func runSendMMS(cmd *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}

	if sendMMSFlagImage == "" {
		return fmt.Errorf("이미지 파일(--image)을 지정하세요")
	}
	if sendFlagTo == "" {
		return fmt.Errorf("수신번호(--to)를 입력하세요")
	}
	if sendFlagText == "" {
		return fmt.Errorf("메시지 내용(--text)을 입력하세요")
	}

	from, err := resolveFrom(c)
	if err != nil {
		return err
	}

	imageID, err := uploadImage(c, sendMMSFlagImage, "MMS")
	if err != nil {
		return err
	}

	msgs, err := buildMessagesFromFlags(func(to string) types.Message {
		return types.Message{
			To:      to,
			From:    from,
			Type:    "MMS",
			Text:    sendFlagText,
			Subject: sendMMSFlagSubject,
			ImageID: imageID,
		}
	})
	if err != nil {
		return err
	}

	return sendMessages(c, msgs)
}

// uploadImage reads a local file, base64-encodes it, and uploads to the SOLAPI
// storage endpoint, returning the file ID.
func uploadImage(c *client.Client, filePath, fileType string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("이미지 파일 읽기 실패: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	req := types.UploadFileRequest{
		File: encoded,
		Type: fileType,
		Name: filepath.Base(filePath),
	}

	raw, err := c.Post(ctx(), "storage/v1/files", req)
	if err != nil {
		return "", fmt.Errorf("이미지 업로드 실패: %w", err)
	}

	var resp types.UploadFileResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("업로드 응답 파싱 실패: %w", err)
	}

	if resp.FileID == "" {
		return "", fmt.Errorf("업로드 응답에 fileId가 없습니다")
	}

	return resp.FileID, nil
}
