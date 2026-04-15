package cmd

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/solapi/solactl/pkg/types"
)

func resetKakaoBrandTemplateFlags() {
	resetFlags()
	kakaoBtplListFlagName = ""
	kakaoBtplListFlagChannelID = ""
	kakaoBtplListFlagChannelGroupID = ""
	kakaoBtplListFlagBrandTemplateID = ""
	kakaoBtplListFlagChatBubbleType = ""
	kakaoBtplListFlagStatus = ""
	kakaoBtplListFlagStartKey = ""
	kakaoBtplListFlagLimit = 20
	kakaoBtplCreateFlagChannelID = ""
	kakaoBtplCreateFlagChannelGroupID = ""
	kakaoBtplCreateFlagChatBubbleType = ""
	kakaoBtplCreateFlagName = ""
	kakaoBtplCreateFlagContent = ""
	kakaoBtplCreateFlagAdult = false
	kakaoBtplCreateFlagHeader = ""
	kakaoBtplCreateFlagImageID = ""
	kakaoBtplCreateFlagImageLink = ""
	kakaoBtplCreateFlagAdditional = ""
	kakaoBtplCreateFlagCarousel = ""
	kakaoBtplCreateFlagMainWideItem = ""
	kakaoBtplCreateFlagSubWideItemList = ""
	kakaoBtplCreateFlagVideo = ""
	kakaoBtplCreateFlagCommerce = ""
	kakaoBtplCreateFlagButtons = ""
	kakaoBtplCreateFlagCoupon = ""
	kakaoBtplUpdateFlagChatBubbleType = ""
	kakaoBtplUpdateFlagName = ""
	kakaoBtplUpdateFlagContent = ""
	kakaoBtplUpdateFlagAdult = false
	kakaoBtplUpdateFlagHeader = ""
	kakaoBtplUpdateFlagImageID = ""
	kakaoBtplUpdateFlagImageLink = ""
	kakaoBtplUpdateFlagAdditional = ""
	kakaoBtplUpdateFlagCarousel = ""
	kakaoBtplUpdateFlagMainWideItem = ""
	kakaoBtplUpdateFlagSubWideItemList = ""
	kakaoBtplUpdateFlagVideo = ""
	kakaoBtplUpdateFlagCommerce = ""
	kakaoBtplUpdateFlagButtons = ""
	kakaoBtplUpdateFlagCoupon = ""
	kakaoBtplDeleteFlagYes = false
	kakaoBtplSendableFlagChannelID = ""
	kakaoBtplSendableFlagBrandTemplateID = ""
}

func setupKakaoBrandTemplateTest(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	resetKakaoBrandTemplateFlags()
	setupKakaoTest(t, handler)
}

// --- Brand Template List Tests ---

func TestKakaoBrandTemplateList_Success(t *testing.T) {
	setupKakaoBrandTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		resp := types.KakaoBrandTemplateListResponse{
			BrandTemplateList: []types.KakaoBrandTemplate{
				{BrandTemplateID: "FAKE_BT_001", Name: "브랜드 1", ChatBubbleType: "TEXT", Status: "ACTIVE", DateCreated: "2025-01-01T00:00:00Z"},
				{BrandTemplateID: "FAKE_BT_002", Name: "브랜드 2", ChatBubbleType: "IMAGE", Status: "ACTIVE", DateCreated: "2025-02-01T00:00:00Z"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "brand-template", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "FAKE_BT_001") {
		t.Error("expected FAKE_BT_001")
	}
	if !strings.Contains(output, "TEXT") {
		t.Error("expected TEXT bubble type")
	}
}

func TestKakaoBrandTemplateList_Pagination(t *testing.T) {
	setupKakaoBrandTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		resp := types.KakaoBrandTemplateListResponse{
			BrandTemplateList: []types.KakaoBrandTemplate{
				{BrandTemplateID: "FAKE_BT_001", Name: "B1", ChatBubbleType: "TEXT", Status: "ACTIVE", DateCreated: "2025-01-01T00:00:00Z"},
			},
			NextKey: "bt_next_key",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	captureBuf(t)
	errBuf := captureErrBuf(t)
	rootCmd.SetArgs([]string{"kakao", "brand-template", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(errBuf.String(), "bt_next_key") {
		t.Error("expected nextKey in pagination hint")
	}
}

func TestKakaoBrandTemplateList_ChannelIDMappedToPfId(t *testing.T) {
	setupKakaoBrandTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("pfId") != "FAKE_CH_001" {
			t.Errorf("expected pfId=FAKE_CH_001, got %s", r.URL.Query().Get("pfId"))
		}
		resp := types.KakaoBrandTemplateListResponse{BrandTemplateList: []types.KakaoBrandTemplate{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "brand-template", "list", "--channel-id", "FAKE_CH_001"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKakaoBrandTemplateList_Alias(t *testing.T) {
	setupKakaoBrandTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		resp := types.KakaoBrandTemplateListResponse{BrandTemplateList: []types.KakaoBrandTemplate{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "btpl", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error with alias 'btpl': %v", err)
	}
}

// --- Brand Template Create Tests ---

func TestKakaoBrandTemplateCreate_Success(t *testing.T) {
	setupKakaoBrandTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]interface{}
		_ = json.Unmarshal(body, &parsed)
		if parsed["pfId"] != "FAKE_CH_001" {
			t.Errorf("expected pfId=FAKE_CH_001, got %v", parsed["pfId"])
		}
		if parsed["chatBubbleType"] != "TEXT" {
			t.Errorf("expected chatBubbleType=TEXT")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"brandTemplateId":"FAKE_BT_NEW","status":"ACTIVE"}`))
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{
		"kakao", "brand-template", "create",
		"--channel-id", "FAKE_CH_001",
		"--chat-bubble-type", "TEXT",
		"--content", "내용입니다",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKakaoBrandTemplateCreate_XOR_BothProvided(t *testing.T) {
	setupKakaoBrandTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{
		"kakao", "brand-template", "create",
		"--channel-id", "FAKE_CH_001",
		"--channel-group-id", "FAKE_GRP_001",
		"--chat-bubble-type", "TEXT",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when both provided")
	}
	if !strings.Contains(err.Error(), "동시에 사용할 수 없습니다") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestKakaoBrandTemplateCreate_XOR_NeitherProvided(t *testing.T) {
	setupKakaoBrandTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{
		"kakao", "brand-template", "create",
		"--chat-bubble-type", "TEXT",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when neither provided")
	}
}

func TestKakaoBrandTemplateCreate_MissingChatBubbleType(t *testing.T) {
	setupKakaoBrandTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{
		"kakao", "brand-template", "create",
		"--channel-id", "FAKE_CH_001",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing chat-bubble-type")
	}
	if !strings.Contains(err.Error(), "--chat-bubble-type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestKakaoBrandTemplateCreate_InvalidButtonsJSON(t *testing.T) {
	setupKakaoBrandTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{
		"kakao", "brand-template", "create",
		"--channel-id", "FAKE_CH_001",
		"--chat-bubble-type", "TEXT",
		"--buttons", "not json",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "JSON 파싱 실패") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Brand Template Update Tests ---

func TestKakaoBrandTemplateUpdate_Success(t *testing.T) {
	setupKakaoBrandTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "FAKE_BT_001") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"brandTemplateId":"FAKE_BT_001"}`))
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{
		"kakao", "brand-template", "update", "FAKE_BT_001",
		"--chat-bubble-type", "TEXT",
		"--content", "새 내용",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "수정되었습니다") {
		t.Error("expected success message")
	}
}

func TestKakaoBrandTemplateUpdate_MissingChatBubbleType(t *testing.T) {
	setupKakaoBrandTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "brand-template", "update", "FAKE_BT_001", "--content", "c"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing chat-bubble-type")
	}
}

// --- Brand Template Delete Tests ---

func TestKakaoBrandTemplateDelete_WithYes(t *testing.T) {
	setupKakaoBrandTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "brand-template", "delete", "FAKE_BT_001", "--yes"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "삭제되었습니다") {
		t.Error("expected success message")
	}
}

func TestKakaoBrandTemplateDelete_ConfirmYes(t *testing.T) {
	setupKakaoBrandTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	buf := captureBuf(t)
	stdinReader = bufio.NewReader(strings.NewReader("y\n"))
	t.Cleanup(func() { stdinReader = nil })

	rootCmd.SetArgs([]string{"kakao", "brand-template", "delete", "FAKE_BT_001"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "삭제되었습니다") {
		t.Error("expected success message")
	}
}

func TestKakaoBrandTemplateDelete_ConfirmNo(t *testing.T) {
	setupKakaoBrandTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not call API when user rejects")
	})

	buf := captureBuf(t)
	stdinReader = bufio.NewReader(strings.NewReader("n\n"))
	t.Cleanup(func() { stdinReader = nil })

	rootCmd.SetArgs([]string{"kakao", "brand-template", "delete", "FAKE_BT_001"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "취소되었습니다") {
		t.Error("expected cancellation message")
	}
}

func TestKakaoBrandTemplateDelete_APIError(t *testing.T) {
	setupKakaoBrandTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"errorCode":"NotAnActiveTemplate","errorMessage":"활성화된 템플릿이 아닙니다."}`))
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "brand-template", "delete", "FAKE_BT_001", "--yes"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for 409")
	}
}

// --- Brand Template Sendable Tests ---

func TestKakaoBrandTemplateSendable_Success(t *testing.T) {
	setupKakaoBrandTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/sendable") {
			t.Errorf("expected /sendable in path, got %s", r.URL.Path)
		}
		templates := []types.KakaoBrandTemplate{
			{BrandTemplateID: "FAKE_BT_001", Name: "발송 가능", ChatBubbleType: "TEXT", Status: "ACTIVE"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(templates)
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "brand-template", "sendable"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "FAKE_BT_001") {
		t.Error("expected template in sendable list")
	}
}

func TestKakaoBrandTemplateSendable_ChannelIDMappedToPfId(t *testing.T) {
	setupKakaoBrandTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("pfId") != "FAKE_CH_001" {
			t.Errorf("expected pfId=FAKE_CH_001, got %s", r.URL.Query().Get("pfId"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]types.KakaoBrandTemplate{})
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "brand-template", "sendable", "--channel-id", "FAKE_CH_001"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
