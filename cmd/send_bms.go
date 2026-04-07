package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/types"
)

var sendBMSCmd = &cobra.Command{
	Use:   "bms",
	Short: "카카오 BMS 메시지를 발송합니다",
	Long:  "카카오 비즈메시지를 발송합니다. 기본은 템플릿 모드이며, --free 플래그로 자유형 모드를 사용합니다.",
	RunE:  runSendBMS,
}

var (
	sendBMSFlagPfID       string
	sendBMSFlagTemplateID string
	sendBMSFlagVariables  string
	sendBMSFlagFree       bool
	sendBMSFlagBubbleType string
	sendBMSFlagTargeting  string
	sendBMSFlagAd         bool
	sendBMSFlagAdult      bool
	sendBMSFlagImage      string
	sendBMSFlagButtonName string
	sendBMSFlagButtonType string
	sendBMSFlagButtonURL  string
	sendBMSFlagButtons    string
)

// validBubbleTypes lists BMS chatBubbleType values the CLI can fully encode.
// Complex types (WIDE_ITEM_LIST, COMMERCE, CAROUSEL_*, PREMIUM_VIDEO) require
// structured fields (carousel, commerce, video, etc.) not exposed as CLI flags,
// so they are excluded from free-mode to avoid guaranteed API rejections.
var validBubbleTypes = map[string]bool{
	"TEXT":  true,
	"IMAGE": true,
	"WIDE":  true,
}

// validTargetingValues is the set of valid BMS targeting values.
var validTargetingValues = map[string]bool{
	"I": true,
	"M": true,
	"N": true,
}

func init() {
	sendBMSCmd.Flags().StringVar(&sendBMSFlagPfID, "pfid", "", "카카오 채널 ID (필수)")
	sendBMSCmd.Flags().StringVar(&sendBMSFlagTemplateID, "template-id", "", "BMS 템플릿 ID")
	sendBMSCmd.Flags().StringVar(&sendBMSFlagVariables, "variables", "", "변수 (JSON 문자열)")
	sendBMSCmd.Flags().BoolVar(&sendBMSFlagFree, "free", false, "자유형 모드")
	sendBMSCmd.Flags().StringVar(&sendBMSFlagBubbleType, "bubble-type", "", "chatBubbleType (자유형 필수: TEXT, IMAGE, WIDE 등)")
	sendBMSCmd.Flags().StringVar(&sendBMSFlagTargeting, "targeting", "", "타겟팅 (I: 개인, M: 모수, N: 신규)")
	sendBMSCmd.Flags().BoolVar(&sendBMSFlagAd, "ad", false, "광고성 메시지")
	sendBMSCmd.Flags().BoolVar(&sendBMSFlagAdult, "adult", false, "성인 콘텐츠")
	sendBMSCmd.Flags().StringVar(&sendBMSFlagImage, "image", "", "이미지 파일 경로")
	sendBMSCmd.Flags().StringVar(&sendBMSFlagButtonName, "button-name", "", "버튼 이름 (단일 버튼)")
	sendBMSCmd.Flags().StringVar(&sendBMSFlagButtonType, "button-type", "", "버튼 타입 (WL, AL 등)")
	sendBMSCmd.Flags().StringVar(&sendBMSFlagButtonURL, "button-url", "", "버튼 URL")
	sendBMSCmd.Flags().StringVar(&sendBMSFlagButtons, "buttons", "", "버튼 목록 (JSON 배열)")
	sendCmd.AddCommand(sendBMSCmd)
}

func runSendBMS(cmd *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}

	if sendFlagTo == "" && sendFlagFile == "" {
		return fmt.Errorf("수신번호(--to)를 입력하세요")
	}
	if sendBMSFlagPfID == "" {
		return fmt.Errorf("카카오 채널 ID(--pfid)를 입력하세요")
	}
	if sendBMSFlagTargeting == "" {
		return fmt.Errorf("targeting(--targeting)을 입력하세요 (I, M, N 중 선택)")
	}

	targeting := strings.ToUpper(sendBMSFlagTargeting)
	if !validTargetingValues[targeting] {
		return fmt.Errorf("유효하지 않은 targeting 값: %s (I, M, N 중 선택)", sendBMSFlagTargeting)
	}

	variables, err := parseVariables(sendBMSFlagVariables)
	if err != nil {
		return err
	}

	kakaoOpts := &types.KakaoOptions{
		PfID:      sendBMSFlagPfID,
		Variables: variables,
		BMS: &types.KakaoBMSOptions{
			Targeting: targeting,
		},
	}
	if sendBMSFlagAd {
		kakaoOpts.AdFlag = boolPtr(true)
	}

	if sendBMSFlagFree {
		// Free mode: --bubble-type required, --template-id not needed
		if sendBMSFlagBubbleType == "" {
			return fmt.Errorf("chatBubbleType(--bubble-type)을 입력하세요")
		}
		bubbleType := strings.ToUpper(sendBMSFlagBubbleType)
		if !validBubbleTypes[bubbleType] {
			return fmt.Errorf("유효하지 않은 chatBubbleType: %s", sendBMSFlagBubbleType)
		}
		kakaoOpts.BMS.ChatBubbleType = bubbleType

		// Validate required fields per bubble type
		if sendFlagText == "" {
			return fmt.Errorf("메시지 내용(--text)을 입력하세요")
		}
		if (bubbleType == "IMAGE" || bubbleType == "WIDE") && sendBMSFlagImage == "" {
			return fmt.Errorf("%s 타입은 이미지(--image)가 필수입니다", bubbleType)
		}

		if sendBMSFlagAdult {
			kakaoOpts.BMS.Adult = boolPtr(true)
		}

		buttons, err := buildBMSButtons()
		if err != nil {
			return err
		}
		kakaoOpts.Buttons = buttons

		if sendBMSFlagImage != "" {
			imageType := "KAKAO"
			if bubbleType == "WIDE" {
				imageType = "BMS_WIDE"
			}
			imageID, err := uploadImage(c, sendBMSFlagImage, imageType)
			if err != nil {
				return err
			}
			kakaoOpts.ImageID = imageID
		}
	} else {
		// Template mode: --template-id required
		if sendBMSFlagTemplateID == "" {
			return fmt.Errorf("BMS 템플릿 ID(--template-id)를 입력하세요 (자유형은 --free 사용)")
		}
		kakaoOpts.TemplateID = sendBMSFlagTemplateID

		// Template-based BMS_IMAGE/BMS_WIDE templates also need imageId
		if sendBMSFlagImage != "" {
			imageID, err := uploadImage(c, sendBMSFlagImage, "KAKAO")
			if err != nil {
				return err
			}
			kakaoOpts.ImageID = imageID
		}
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

// buildBMSButtons constructs a button slice from either --buttons JSON or
// the --button-name/--button-type/--button-url convenience flags.
func buildBMSButtons() ([]types.KakaoButton, error) {
	if sendBMSFlagButtons != "" && sendBMSFlagButtonName != "" {
		return nil, fmt.Errorf("--buttons와 --button-name은 동시에 사용할 수 없습니다")
	}

	if sendBMSFlagButtons != "" {
		return parseKakaoButtons(sendBMSFlagButtons)
	}

	if sendBMSFlagButtonName != "" {
		btn := types.KakaoButton{
			ButtonName: sendBMSFlagButtonName,
			ButtonType: sendBMSFlagButtonType,
			LinkMo:     sendBMSFlagButtonURL,
		}
		return []types.KakaoButton{btn}, nil
	}

	return nil, nil
}
