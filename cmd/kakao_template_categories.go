package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/types"
)

var kakaoTemplateCategoriesCmd = &cobra.Command{
	Use:   "categories",
	Short: "알림톡 템플릿 카테고리 목록을 조회합니다",
	RunE:  runKakaoTemplateCategories,
}

func init() {
	kakaoTemplateCmd.AddCommand(kakaoTemplateCategoriesCmd)
}

func runKakaoTemplateCategories(cmd *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}

	raw, err := c.Get(ctx(), "kakao/v2/templates/categories", nil)
	if err != nil {
		return fmt.Errorf("알림톡 템플릿 카테고리 조회 실패: %w", err)
	}

	p := printer()
	if flagJSON {
		return p.PrintJSON(raw)
	}

	var categories []types.KakaoTemplateCategory
	if err := json.Unmarshal(raw, &categories); err != nil {
		return fmt.Errorf("응답 파싱 실패: %w", err)
	}

	headers := []string{"CODE", "NAME"}
	rows := make([][]string, 0, len(categories))
	for i := range categories {
		cat := &categories[i]
		rows = append(rows, []string{cat.Code, cat.Name})
	}
	p.FormatTable(headers, rows)

	return nil
}
