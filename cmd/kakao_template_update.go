package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	kakaoTplUpdateFlagChannelID      string
	kakaoTplUpdateFlagChannelGroupID string
	kakaoTplUpdateFlagName           string
	kakaoTplUpdateFlagContent        string
	kakaoTplUpdateFlagCategoryCode   string
	kakaoTplUpdateFlagButtons        string
	kakaoTplUpdateFlagQuickReplies   string
	kakaoTplUpdateFlagMessageType    string
	kakaoTplUpdateFlagEmphasizeType  string
	kakaoTplUpdateFlagHeader         string
	kakaoTplUpdateFlagHighlight      string
	kakaoTplUpdateFlagItem           string
	kakaoTplUpdateFlagExtra          string
	kakaoTplUpdateFlagAd             string
	kakaoTplUpdateFlagEmphasizeTitle string
	kakaoTplUpdateFlagEmphasizeSub   string
	kakaoTplUpdateFlagSecurityFlag   bool
	kakaoTplUpdateFlagImageID        string
)

var kakaoTemplateUpdateCmd = &cobra.Command{
	Use:   "update <templateId>",
	Short: "알림톡 템플릿을 수정합니다",
	Long: `알림톡 템플릿을 수정합니다. 변경할 필드만 지정하면 됩니다.

사용법:
  solactl kakao template update KA01TP240101000000000000000000000 --name "새 이름"
  solactl kakao template update KA01TP240101000000000000000000000 --content "새 내용" --buttons '[]'`,
	Args: cobra.ExactArgs(1),
	RunE: runKakaoTemplateUpdate,
}

func init() {
	kakaoTemplateUpdateCmd.Flags().StringVar(&kakaoTplUpdateFlagChannelID, "channel-id", "", "채널 ID")
	kakaoTemplateUpdateCmd.Flags().StringVar(&kakaoTplUpdateFlagChannelGroupID, "channel-group-id", "", "채널 그룹 ID")
	kakaoTemplateUpdateCmd.Flags().StringVar(&kakaoTplUpdateFlagName, "name", "", "템플릿 이름")
	kakaoTemplateUpdateCmd.Flags().StringVar(&kakaoTplUpdateFlagContent, "content", "", "템플릿 내용")
	kakaoTemplateUpdateCmd.Flags().StringVar(&kakaoTplUpdateFlagCategoryCode, "category-code", "", "카테고리 코드")
	kakaoTemplateUpdateCmd.Flags().StringVar(&kakaoTplUpdateFlagButtons, "buttons", "", "버튼 목록 (JSON 배열)")
	kakaoTemplateUpdateCmd.Flags().StringVar(&kakaoTplUpdateFlagQuickReplies, "quick-replies", "", "바로연결 목록 (JSON 배열)")
	kakaoTemplateUpdateCmd.Flags().StringVar(&kakaoTplUpdateFlagMessageType, "message-type", "", "메시지 타입 (BA/EX/AD/MI)")
	kakaoTemplateUpdateCmd.Flags().StringVar(&kakaoTplUpdateFlagEmphasizeType, "emphasize-type", "", "강조 타입 (NONE/TEXT/IMAGE/ITEM_LIST)")
	kakaoTemplateUpdateCmd.Flags().StringVar(&kakaoTplUpdateFlagHeader, "header", "", "헤더")
	kakaoTemplateUpdateCmd.Flags().StringVar(&kakaoTplUpdateFlagHighlight, "highlight", "", "하이라이트 (JSON 객체)")
	kakaoTemplateUpdateCmd.Flags().StringVar(&kakaoTplUpdateFlagItem, "item", "", "아이템 리스트 (JSON 객체)")
	kakaoTemplateUpdateCmd.Flags().StringVar(&kakaoTplUpdateFlagExtra, "extra", "", "부가 정보")
	kakaoTemplateUpdateCmd.Flags().StringVar(&kakaoTplUpdateFlagAd, "ad", "", "광고 문구")
	kakaoTemplateUpdateCmd.Flags().StringVar(&kakaoTplUpdateFlagEmphasizeTitle, "emphasize-title", "", "강조 제목")
	kakaoTemplateUpdateCmd.Flags().StringVar(&kakaoTplUpdateFlagEmphasizeSub, "emphasize-subtitle", "", "강조 부제목")
	kakaoTemplateUpdateCmd.Flags().BoolVar(&kakaoTplUpdateFlagSecurityFlag, "security-flag", false, "보안 메시지 여부")
	kakaoTemplateUpdateCmd.Flags().StringVar(&kakaoTplUpdateFlagImageID, "image-id", "", "이미지 ID")
	kakaoTemplateCmd.AddCommand(kakaoTemplateUpdateCmd)
}

func runKakaoTemplateUpdate(cmd *cobra.Command, args []string) error {
	templateID := args[0]

	c, err := newClient()
	if err != nil {
		return err
	}

	body := map[string]interface{}{}

	// Only include fields that were explicitly set
	setIfChanged(cmd, body, "name", kakaoTplUpdateFlagName)
	setIfChanged(cmd, body, "content", kakaoTplUpdateFlagContent)
	setIfChanged(cmd, body, "categoryCode", "category-code", kakaoTplUpdateFlagCategoryCode)
	setIfChanged(cmd, body, "messageType", "message-type", kakaoTplUpdateFlagMessageType)
	setIfChanged(cmd, body, "emphasizeType", "emphasize-type", kakaoTplUpdateFlagEmphasizeType)
	setIfChanged(cmd, body, "header", kakaoTplUpdateFlagHeader)
	setIfChanged(cmd, body, "extra", kakaoTplUpdateFlagExtra)
	setIfChanged(cmd, body, "ad", kakaoTplUpdateFlagAd)
	setIfChanged(cmd, body, "emphasizeTitle", "emphasize-title", kakaoTplUpdateFlagEmphasizeTitle)
	setIfChanged(cmd, body, "emphasizeSubtitle", "emphasize-subtitle", kakaoTplUpdateFlagEmphasizeSub)
	setIfChanged(cmd, body, "imageId", "image-id", kakaoTplUpdateFlagImageID)

	if cmd.Flags().Changed("security-flag") {
		body["securityFlag"] = kakaoTplUpdateFlagSecurityFlag
	}

	// JSON fields
	if cmd.Flags().Changed("buttons") {
		if err := setJSONField(body, "buttons", kakaoTplUpdateFlagButtons); err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("quick-replies") {
		if err := setJSONField(body, "quickReplies", kakaoTplUpdateFlagQuickReplies); err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("highlight") {
		if err := setJSONField(body, "highlight", kakaoTplUpdateFlagHighlight); err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("item") {
		if err := setJSONField(body, "item", kakaoTplUpdateFlagItem); err != nil {
			return err
		}
	}

	raw, err := c.Put(ctx(), fmt.Sprintf("kakao/v2/templates/%s", templateID), body)
	if err != nil {
		return fmt.Errorf("알림톡 템플릿 수정 실패: %w", err)
	}

	p := printer()
	if flagJSON {
		return p.PrintJSON(raw)
	}

	_, _ = fmt.Fprintf(out(), "알림톡 템플릿 %s이 수정되었습니다.\n", templateID)
	return nil
}

// setIfChanged adds a string field to body only if the corresponding flag was changed.
// Accepts: (cmd, body, jsonKey, value) or (cmd, body, jsonKey, flagName, value).
func setIfChanged(cmd *cobra.Command, body map[string]interface{}, args ...interface{}) {
	switch len(args) {
	case 2:
		// jsonKey == flagName
		jsonKey := args[0].(string)
		value := args[1].(string)
		if cmd.Flags().Changed(jsonKey) {
			body[jsonKey] = value
		}
	case 3:
		// jsonKey != flagName
		jsonKey := args[0].(string)
		flagName := args[1].(string)
		value := args[2].(string)
		if cmd.Flags().Changed(flagName) {
			body[jsonKey] = value
		}
	}
}
