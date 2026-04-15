package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	kakaoBtplUpdateFlagChatBubbleType  string
	kakaoBtplUpdateFlagName            string
	kakaoBtplUpdateFlagContent         string
	kakaoBtplUpdateFlagAdult           bool
	kakaoBtplUpdateFlagHeader          string
	kakaoBtplUpdateFlagImageID         string
	kakaoBtplUpdateFlagImageLink       string
	kakaoBtplUpdateFlagAdditional      string
	kakaoBtplUpdateFlagCarousel        string
	kakaoBtplUpdateFlagMainWideItem    string
	kakaoBtplUpdateFlagSubWideItemList string
	kakaoBtplUpdateFlagVideo           string
	kakaoBtplUpdateFlagCommerce        string
	kakaoBtplUpdateFlagButtons         string
	kakaoBtplUpdateFlagCoupon          string
)

var kakaoBrandTemplateUpdateCmd = &cobra.Command{
	Use:   "update <brandTemplateId>",
	Short: "브랜드 템플릿을 수정합니다",
	Long: `브랜드 템플릿을 수정합니다.

사용법:
  solactl kakao brand-template update KA01BP240000000000000000000000000 --chat-bubble-type TEXT --content "새 내용"`,
	Args: cobra.ExactArgs(1),
	RunE: runKakaoBrandTemplateUpdate,
}

func init() {
	kakaoBrandTemplateUpdateCmd.Flags().StringVar(&kakaoBtplUpdateFlagChatBubbleType, "chat-bubble-type", "", "말풍선 타입 (필수)")
	kakaoBrandTemplateUpdateCmd.Flags().StringVar(&kakaoBtplUpdateFlagName, "name", "", "이름")
	kakaoBrandTemplateUpdateCmd.Flags().StringVar(&kakaoBtplUpdateFlagContent, "content", "", "내용")
	kakaoBrandTemplateUpdateCmd.Flags().BoolVar(&kakaoBtplUpdateFlagAdult, "adult", false, "성인 콘텐츠")
	kakaoBrandTemplateUpdateCmd.Flags().StringVar(&kakaoBtplUpdateFlagHeader, "header", "", "헤더")
	kakaoBrandTemplateUpdateCmd.Flags().StringVar(&kakaoBtplUpdateFlagImageID, "image-id", "", "이미지 ID")
	kakaoBrandTemplateUpdateCmd.Flags().StringVar(&kakaoBtplUpdateFlagImageLink, "image-link", "", "이미지 링크")
	kakaoBrandTemplateUpdateCmd.Flags().StringVar(&kakaoBtplUpdateFlagAdditional, "additional-content", "", "추가 콘텐츠")
	kakaoBrandTemplateUpdateCmd.Flags().StringVar(&kakaoBtplUpdateFlagCarousel, "carousel", "", "캐러셀 (JSON 객체)")
	kakaoBrandTemplateUpdateCmd.Flags().StringVar(&kakaoBtplUpdateFlagMainWideItem, "main-wide-item", "", "메인 와이드 아이템 (JSON 객체)")
	kakaoBrandTemplateUpdateCmd.Flags().StringVar(&kakaoBtplUpdateFlagSubWideItemList, "sub-wide-item-list", "", "서브 와이드 아이템 목록 (JSON 배열)")
	kakaoBrandTemplateUpdateCmd.Flags().StringVar(&kakaoBtplUpdateFlagVideo, "video", "", "동영상 (JSON 객체)")
	kakaoBrandTemplateUpdateCmd.Flags().StringVar(&kakaoBtplUpdateFlagCommerce, "commerce", "", "커머스 (JSON 객체)")
	kakaoBrandTemplateUpdateCmd.Flags().StringVar(&kakaoBtplUpdateFlagButtons, "buttons", "", "버튼 목록 (JSON 배열)")
	kakaoBrandTemplateUpdateCmd.Flags().StringVar(&kakaoBtplUpdateFlagCoupon, "coupon", "", "쿠폰 (JSON 객체)")
	kakaoBrandTemplateCmd.AddCommand(kakaoBrandTemplateUpdateCmd)
}

func runKakaoBrandTemplateUpdate(cmd *cobra.Command, args []string) error {
	brandTemplateID := args[0]

	if kakaoBtplUpdateFlagChatBubbleType == "" {
		return fmt.Errorf("말풍선 타입(--chat-bubble-type)을 입력하세요")
	}

	c, err := newClient()
	if err != nil {
		return err
	}

	body := map[string]any{
		"chatBubbleType": kakaoBtplUpdateFlagChatBubbleType,
	}

	setStringIfChanged(cmd, body, "name", "name", kakaoBtplUpdateFlagName)
	setStringIfChanged(cmd, body, "content", "content", kakaoBtplUpdateFlagContent)
	setStringIfChanged(cmd, body, "header", "header", kakaoBtplUpdateFlagHeader)
	setStringIfChanged(cmd, body, "imageId", "image-id", kakaoBtplUpdateFlagImageID)
	setStringIfChanged(cmd, body, "imageLink", "image-link", kakaoBtplUpdateFlagImageLink)
	setStringIfChanged(cmd, body, "additionalContent", "additional-content", kakaoBtplUpdateFlagAdditional)

	if cmd.Flags().Changed("adult") {
		body["adult"] = kakaoBtplUpdateFlagAdult
	}

	// JSON fields — only include if explicitly provided
	jsonFields := []struct{ jsonKey, flagName, value string }{
		{"carousel", "carousel", kakaoBtplUpdateFlagCarousel},
		{"mainWideItem", "main-wide-item", kakaoBtplUpdateFlagMainWideItem},
		{"subWideItemList", "sub-wide-item-list", kakaoBtplUpdateFlagSubWideItemList},
		{"video", "video", kakaoBtplUpdateFlagVideo},
		{"commerce", "commerce", kakaoBtplUpdateFlagCommerce},
		{"buttons", "buttons", kakaoBtplUpdateFlagButtons},
		{"coupon", "coupon", kakaoBtplUpdateFlagCoupon},
	}
	for _, f := range jsonFields {
		if err := setJSONIfChanged(cmd, body, f.jsonKey, f.flagName, f.value); err != nil {
			return err
		}
	}

	raw, err := c.Put(ctx(), fmt.Sprintf("kakao/v2/brand-templates/%s", brandTemplateID), body)
	if err != nil {
		return fmt.Errorf("브랜드 템플릿 수정 실패: %w", err)
	}

	p := printer()
	if flagJSON {
		return p.PrintJSON(raw)
	}

	_, _ = fmt.Fprintf(out(), "브랜드 템플릿 %s이 수정되었습니다.\n", brandTemplateID)
	return nil
}
