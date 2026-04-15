package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	kakaoBtplCreateFlagChannelID       string
	kakaoBtplCreateFlagChannelGroupID  string
	kakaoBtplCreateFlagChatBubbleType  string
	kakaoBtplCreateFlagName            string
	kakaoBtplCreateFlagContent         string
	kakaoBtplCreateFlagAdult           bool
	kakaoBtplCreateFlagHeader          string
	kakaoBtplCreateFlagImageID         string
	kakaoBtplCreateFlagImageLink       string
	kakaoBtplCreateFlagAdditional      string
	kakaoBtplCreateFlagCarousel        string
	kakaoBtplCreateFlagMainWideItem    string
	kakaoBtplCreateFlagSubWideItemList string
	kakaoBtplCreateFlagVideo           string
	kakaoBtplCreateFlagCommerce        string
	kakaoBtplCreateFlagButtons         string
	kakaoBtplCreateFlagCoupon          string
)

var kakaoBrandTemplateCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "브랜드 템플릿을 생성합니다",
	Long: `브랜드 템플릿을 생성합니다. 검수 없이 즉시 사용 가능합니다.

사용법:
  solactl kakao brand-template create --channel-id <channelId> --chat-bubble-type TEXT --content "내용"
  solactl kakao brand-template create --channel-id <channelId> --chat-bubble-type IMAGE --content "내용" --image-id <imageId>`,
	RunE: runKakaoBrandTemplateCreate,
}

func init() {
	kakaoBrandTemplateCreateCmd.Flags().StringVar(&kakaoBtplCreateFlagChannelID, "channel-id", "", "채널 ID (--channel-id 또는 --channel-group-id 필수)")
	kakaoBrandTemplateCreateCmd.Flags().StringVar(&kakaoBtplCreateFlagChannelGroupID, "channel-group-id", "", "채널 그룹 ID")
	kakaoBrandTemplateCreateCmd.Flags().StringVar(&kakaoBtplCreateFlagChatBubbleType, "chat-bubble-type", "", "말풍선 타입 (필수: TEXT/IMAGE/WIDE/WIDE_ITEM_LIST/CAROUSEL_FEED/PREMIUM_VIDEO/COMMERCE/CAROUSEL_COMMERCE)")
	kakaoBrandTemplateCreateCmd.Flags().StringVar(&kakaoBtplCreateFlagName, "name", "", "이름")
	kakaoBrandTemplateCreateCmd.Flags().StringVar(&kakaoBtplCreateFlagContent, "content", "", "내용")
	kakaoBrandTemplateCreateCmd.Flags().BoolVar(&kakaoBtplCreateFlagAdult, "adult", false, "성인 콘텐츠")
	kakaoBrandTemplateCreateCmd.Flags().StringVar(&kakaoBtplCreateFlagHeader, "header", "", "헤더")
	kakaoBrandTemplateCreateCmd.Flags().StringVar(&kakaoBtplCreateFlagImageID, "image-id", "", "이미지 ID")
	kakaoBrandTemplateCreateCmd.Flags().StringVar(&kakaoBtplCreateFlagImageLink, "image-link", "", "이미지 링크")
	kakaoBrandTemplateCreateCmd.Flags().StringVar(&kakaoBtplCreateFlagAdditional, "additional-content", "", "추가 콘텐츠")
	kakaoBrandTemplateCreateCmd.Flags().StringVar(&kakaoBtplCreateFlagCarousel, "carousel", "", "캐러셀 (JSON 객체)")
	kakaoBrandTemplateCreateCmd.Flags().StringVar(&kakaoBtplCreateFlagMainWideItem, "main-wide-item", "", "메인 와이드 아이템 (JSON 객체)")
	kakaoBrandTemplateCreateCmd.Flags().StringVar(&kakaoBtplCreateFlagSubWideItemList, "sub-wide-item-list", "", "서브 와이드 아이템 목록 (JSON 배열)")
	kakaoBrandTemplateCreateCmd.Flags().StringVar(&kakaoBtplCreateFlagVideo, "video", "", "동영상 (JSON 객체)")
	kakaoBrandTemplateCreateCmd.Flags().StringVar(&kakaoBtplCreateFlagCommerce, "commerce", "", "커머스 (JSON 객체)")
	kakaoBrandTemplateCreateCmd.Flags().StringVar(&kakaoBtplCreateFlagButtons, "buttons", "", "버튼 목록 (JSON 배열)")
	kakaoBrandTemplateCreateCmd.Flags().StringVar(&kakaoBtplCreateFlagCoupon, "coupon", "", "쿠폰 (JSON 객체)")
	kakaoBrandTemplateCmd.AddCommand(kakaoBrandTemplateCreateCmd)
}

func runKakaoBrandTemplateCreate(cmd *cobra.Command, args []string) error {
	// XOR validation
	if kakaoBtplCreateFlagChannelID == "" && kakaoBtplCreateFlagChannelGroupID == "" {
		return fmt.Errorf("--channel-id 또는 --channel-group-id를 입력하세요")
	}
	if kakaoBtplCreateFlagChannelID != "" && kakaoBtplCreateFlagChannelGroupID != "" {
		return fmt.Errorf("--channel-id와 --channel-group-id는 동시에 사용할 수 없습니다")
	}
	if kakaoBtplCreateFlagChatBubbleType == "" {
		return fmt.Errorf("말풍선 타입(--chat-bubble-type)을 입력하세요")
	}

	c, err := newClient()
	if err != nil {
		return err
	}

	body := map[string]interface{}{
		"chatBubbleType": kakaoBtplCreateFlagChatBubbleType,
	}

	// brand-template API uses pfId/pfGroupId directly
	if kakaoBtplCreateFlagChannelID != "" {
		body["pfId"] = kakaoBtplCreateFlagChannelID
	}
	if kakaoBtplCreateFlagChannelGroupID != "" {
		body["pfGroupId"] = kakaoBtplCreateFlagChannelGroupID
	}

	if kakaoBtplCreateFlagName != "" {
		body["name"] = kakaoBtplCreateFlagName
	}
	if kakaoBtplCreateFlagContent != "" {
		body["content"] = kakaoBtplCreateFlagContent
	}
	if kakaoBtplCreateFlagAdult {
		body["adult"] = true
	}
	if kakaoBtplCreateFlagHeader != "" {
		body["header"] = kakaoBtplCreateFlagHeader
	}
	if kakaoBtplCreateFlagImageID != "" {
		body["imageId"] = kakaoBtplCreateFlagImageID
	}
	if kakaoBtplCreateFlagImageLink != "" {
		body["imageLink"] = kakaoBtplCreateFlagImageLink
	}
	if kakaoBtplCreateFlagAdditional != "" {
		body["additionalContent"] = kakaoBtplCreateFlagAdditional
	}

	// JSON fields
	if err := setJSONField(body, "carousel", kakaoBtplCreateFlagCarousel); err != nil {
		return err
	}
	if err := setJSONField(body, "mainWideItem", kakaoBtplCreateFlagMainWideItem); err != nil {
		return err
	}
	if err := setJSONField(body, "subWideItemList", kakaoBtplCreateFlagSubWideItemList); err != nil {
		return err
	}
	if err := setJSONField(body, "video", kakaoBtplCreateFlagVideo); err != nil {
		return err
	}
	if err := setJSONField(body, "commerce", kakaoBtplCreateFlagCommerce); err != nil {
		return err
	}
	if err := setJSONField(body, "buttons", kakaoBtplCreateFlagButtons); err != nil {
		return err
	}
	if err := setJSONField(body, "coupon", kakaoBtplCreateFlagCoupon); err != nil {
		return err
	}

	raw, err := c.Post(ctx(), "kakao/v2/brand-templates", body)
	if err != nil {
		return fmt.Errorf("브랜드 템플릿 생성 실패: %w", err)
	}

	p := printer()
	if flagJSON {
		return p.PrintJSON(raw)
	}

	_, _ = fmt.Fprintln(out(), "브랜드 템플릿이 생성되었습니다.")
	return nil
}
