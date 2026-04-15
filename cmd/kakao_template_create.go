package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	kakaoTplCreateFlagChannelID      string
	kakaoTplCreateFlagChannelGroupID string
	kakaoTplCreateFlagName           string
	kakaoTplCreateFlagContent        string
	kakaoTplCreateFlagCategoryCode   string
	kakaoTplCreateFlagButtons        string
	kakaoTplCreateFlagQuickReplies   string
	kakaoTplCreateFlagMessageType    string
	kakaoTplCreateFlagEmphasizeType  string
	kakaoTplCreateFlagHeader         string
	kakaoTplCreateFlagHighlight      string
	kakaoTplCreateFlagItem           string
	kakaoTplCreateFlagExtra          string
	kakaoTplCreateFlagAd             string
	kakaoTplCreateFlagEmphasizeTitle string
	kakaoTplCreateFlagEmphasizeSub   string
	kakaoTplCreateFlagSecurityFlag   bool
	kakaoTplCreateFlagImageID        string
)

var kakaoTemplateCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "알림톡 템플릿을 생성합니다",
	Long: `알림톡 템플릿을 생성합니다.

사용법:
  solactl kakao template create --channel-id <channelId> --name "주문 확인" --content "#{고객명}님 주문이 완료되었습니다." --category-code 001001

  solactl kakao template create --channel-group-id <groupId> --name "배송 알림" --content "배송이 시작되었습니다." --category-code 002001 --buttons '[{"buttonType":"WL","buttonName":"조회","linkMo":"https://example.com"}]'`,
	RunE: runKakaoTemplateCreate,
}

func init() {
	kakaoTemplateCreateCmd.Flags().StringVar(&kakaoTplCreateFlagChannelID, "channel-id", "", "채널 ID (--channel-id 또는 --channel-group-id 필수)")
	kakaoTemplateCreateCmd.Flags().StringVar(&kakaoTplCreateFlagChannelGroupID, "channel-group-id", "", "채널 그룹 ID")
	kakaoTemplateCreateCmd.Flags().StringVar(&kakaoTplCreateFlagName, "name", "", "템플릿 이름 (필수)")
	kakaoTemplateCreateCmd.Flags().StringVar(&kakaoTplCreateFlagContent, "content", "", "템플릿 내용 (필수)")
	kakaoTemplateCreateCmd.Flags().StringVar(&kakaoTplCreateFlagCategoryCode, "category-code", "", "카테고리 코드 (필수)")
	kakaoTemplateCreateCmd.Flags().StringVar(&kakaoTplCreateFlagButtons, "buttons", "", "버튼 목록 (JSON 배열)")
	kakaoTemplateCreateCmd.Flags().StringVar(&kakaoTplCreateFlagQuickReplies, "quick-replies", "", "바로연결 목록 (JSON 배열)")
	kakaoTemplateCreateCmd.Flags().StringVar(&kakaoTplCreateFlagMessageType, "message-type", "", "메시지 타입 (BA/EX/AD/MI)")
	kakaoTemplateCreateCmd.Flags().StringVar(&kakaoTplCreateFlagEmphasizeType, "emphasize-type", "", "강조 타입 (NONE/TEXT/IMAGE/ITEM_LIST)")
	kakaoTemplateCreateCmd.Flags().StringVar(&kakaoTplCreateFlagHeader, "header", "", "헤더")
	kakaoTemplateCreateCmd.Flags().StringVar(&kakaoTplCreateFlagHighlight, "highlight", "", "하이라이트 (JSON 객체)")
	kakaoTemplateCreateCmd.Flags().StringVar(&kakaoTplCreateFlagItem, "item", "", "아이템 리스트 (JSON 객체)")
	kakaoTemplateCreateCmd.Flags().StringVar(&kakaoTplCreateFlagExtra, "extra", "", "부가 정보")
	kakaoTemplateCreateCmd.Flags().StringVar(&kakaoTplCreateFlagAd, "ad", "", "광고 문구")
	kakaoTemplateCreateCmd.Flags().StringVar(&kakaoTplCreateFlagEmphasizeTitle, "emphasize-title", "", "강조 제목")
	kakaoTemplateCreateCmd.Flags().StringVar(&kakaoTplCreateFlagEmphasizeSub, "emphasize-subtitle", "", "강조 부제목")
	kakaoTemplateCreateCmd.Flags().BoolVar(&kakaoTplCreateFlagSecurityFlag, "security-flag", false, "보안 메시지 여부")
	kakaoTemplateCreateCmd.Flags().StringVar(&kakaoTplCreateFlagImageID, "image-id", "", "이미지 ID")
	kakaoTemplateCmd.AddCommand(kakaoTemplateCreateCmd)
}

func runKakaoTemplateCreate(cmd *cobra.Command, args []string) error {
	// XOR validation: exactly one of channel-id or channel-group-id required
	if kakaoTplCreateFlagChannelID == "" && kakaoTplCreateFlagChannelGroupID == "" {
		return fmt.Errorf("--channel-id 또는 --channel-group-id를 입력하세요")
	}
	if kakaoTplCreateFlagChannelID != "" && kakaoTplCreateFlagChannelGroupID != "" {
		return fmt.Errorf("--channel-id와 --channel-group-id는 동시에 사용할 수 없습니다")
	}
	if kakaoTplCreateFlagName == "" {
		return fmt.Errorf("템플릿 이름(--name)을 입력하세요")
	}
	if kakaoTplCreateFlagContent == "" {
		return fmt.Errorf("템플릿 내용(--content)을 입력하세요")
	}
	if kakaoTplCreateFlagCategoryCode == "" {
		return fmt.Errorf("카테고리 코드(--category-code)를 입력하세요")
	}

	c, err := newClient()
	if err != nil {
		return err
	}

	body := map[string]any{
		"name":         kakaoTplCreateFlagName,
		"content":      kakaoTplCreateFlagContent,
		"categoryCode": kakaoTplCreateFlagCategoryCode,
	}

	if kakaoTplCreateFlagChannelID != "" {
		body["channelId"] = kakaoTplCreateFlagChannelID
	}
	if kakaoTplCreateFlagChannelGroupID != "" {
		body["channelGroupId"] = kakaoTplCreateFlagChannelGroupID
	}

	// Optional JSON fields
	if err := setJSONField(body, "buttons", kakaoTplCreateFlagButtons); err != nil {
		return err
	}
	if err := setJSONField(body, "quickReplies", kakaoTplCreateFlagQuickReplies); err != nil {
		return err
	}
	if err := setJSONField(body, "highlight", kakaoTplCreateFlagHighlight); err != nil {
		return err
	}
	if err := setJSONField(body, "item", kakaoTplCreateFlagItem); err != nil {
		return err
	}

	// Optional string fields
	if kakaoTplCreateFlagMessageType != "" {
		body["messageType"] = kakaoTplCreateFlagMessageType
	}
	if kakaoTplCreateFlagEmphasizeType != "" {
		body["emphasizeType"] = kakaoTplCreateFlagEmphasizeType
	}
	if kakaoTplCreateFlagHeader != "" {
		body["header"] = kakaoTplCreateFlagHeader
	}
	if kakaoTplCreateFlagExtra != "" {
		body["extra"] = kakaoTplCreateFlagExtra
	}
	if kakaoTplCreateFlagAd != "" {
		body["ad"] = kakaoTplCreateFlagAd
	}
	if kakaoTplCreateFlagEmphasizeTitle != "" {
		body["emphasizeTitle"] = kakaoTplCreateFlagEmphasizeTitle
	}
	if kakaoTplCreateFlagEmphasizeSub != "" {
		body["emphasizeSubtitle"] = kakaoTplCreateFlagEmphasizeSub
	}
	if kakaoTplCreateFlagImageID != "" {
		body["imageId"] = kakaoTplCreateFlagImageID
	}
	if kakaoTplCreateFlagSecurityFlag {
		body["securityFlag"] = true
	}

	raw, err := c.Post(ctx(), "kakao/v2/templates", body)
	if err != nil {
		return fmt.Errorf("알림톡 템플릿 생성 실패: %w", err)
	}

	p := printer()
	if flagJSON {
		return p.PrintJSON(raw)
	}

	_, _ = fmt.Fprintln(out(), "알림톡 템플릿이 생성되었습니다.")
	return nil
}

// setJSONField parses a JSON string and sets it in the body map.
// Returns nil for empty input, error for invalid JSON.
func setJSONField(body map[string]any, key, value string) error {
	if value == "" {
		return nil
	}
	var parsed json.RawMessage
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		return fmt.Errorf("%s JSON 파싱 실패: %w", key, err)
	}
	body[key] = parsed
	return nil
}
