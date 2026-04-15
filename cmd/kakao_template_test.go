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

func resetKakaoTemplateFlags() {
	resetFlags()
	kakaoTplListFlagName = ""
	kakaoTplListFlagChannelID = ""
	kakaoTplListFlagChannelGroupID = ""
	kakaoTplListFlagTemplateID = ""
	kakaoTplListFlagStatus = ""
	kakaoTplListFlagIsHidden = false
	kakaoTplListFlagIsMine = false
	kakaoTplListFlagStartKey = ""
	kakaoTplListFlagLimit = 20
	kakaoTplCreateFlagChannelID = ""
	kakaoTplCreateFlagChannelGroupID = ""
	kakaoTplCreateFlagName = ""
	kakaoTplCreateFlagContent = ""
	kakaoTplCreateFlagCategoryCode = ""
	kakaoTplCreateFlagButtons = ""
	kakaoTplCreateFlagQuickReplies = ""
	kakaoTplCreateFlagMessageType = ""
	kakaoTplCreateFlagEmphasizeType = ""
	kakaoTplCreateFlagHeader = ""
	kakaoTplCreateFlagHighlight = ""
	kakaoTplCreateFlagItem = ""
	kakaoTplCreateFlagExtra = ""
	kakaoTplCreateFlagAd = ""
	kakaoTplCreateFlagEmphasizeTitle = ""
	kakaoTplCreateFlagEmphasizeSub = ""
	kakaoTplCreateFlagSecurityFlag = false
	kakaoTplCreateFlagImageID = ""
	kakaoTplUpdateFlagName = ""
	kakaoTplUpdateFlagContent = ""
	kakaoTplUpdateFlagCategoryCode = ""
	kakaoTplUpdateFlagButtons = ""
	kakaoTplUpdateFlagQuickReplies = ""
	kakaoTplUpdateFlagMessageType = ""
	kakaoTplUpdateFlagEmphasizeType = ""
	kakaoTplUpdateFlagHeader = ""
	kakaoTplUpdateFlagHighlight = ""
	kakaoTplUpdateFlagItem = ""
	kakaoTplUpdateFlagExtra = ""
	kakaoTplUpdateFlagAd = ""
	kakaoTplUpdateFlagEmphasizeTitle = ""
	kakaoTplUpdateFlagEmphasizeSub = ""
	kakaoTplUpdateFlagSecurityFlag = false
	kakaoTplUpdateFlagImageID = ""
	kakaoTplDeleteFlagYes = false
	kakaoTplInspectFlagComment = ""
	kakaoTplSendableFlagChannelID = ""
	kakaoTplSendableFlagTemplateID = ""
	kakaoTplSendableFlagSenderKey = ""
	kakaoTplSendableFlagTemplateCode = ""
}

func setupKakaoTemplateTest(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	resetKakaoTemplateFlags()
	setupKakaoTest(t, handler)
}

// --- Template List Tests ---

func TestKakaoTemplateList_Success(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		resp := types.KakaoTemplateListResponse{
			TemplateList: []types.KakaoTemplate{
				{TemplateID: "FAKE_TPL_001", Name: "주문 확인", Status: "APPROVED", MessageType: "BA", ChannelID: "FAKE_CH_001", DateCreated: "2025-01-01T00:00:00Z"},
				{TemplateID: "FAKE_TPL_002", Name: "배송 알림", Status: "PENDING", MessageType: "EX", ChannelGroupID: "FAKE_GRP_001", DateCreated: "2025-02-01T00:00:00Z"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "template", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "FAKE_TPL_001") {
		t.Error("expected FAKE_TPL_001")
	}
	if !strings.Contains(output, "주문 확인") {
		t.Error("expected template name")
	}
	if !strings.Contains(output, "APPROVED") {
		t.Error("expected status APPROVED")
	}
}

func TestKakaoTemplateList_Pagination(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		resp := types.KakaoTemplateListResponse{
			TemplateList: []types.KakaoTemplate{
				{TemplateID: "FAKE_TPL_001", Name: "T1", Status: "APPROVED", DateCreated: "2025-01-01T00:00:00Z"},
			},
			NextKey: "tpl_next_key",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	captureBuf(t)
	errBuf := captureErrBuf(t)
	rootCmd.SetArgs([]string{"kakao", "template", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(errBuf.String(), "tpl_next_key") {
		t.Error("expected nextKey in pagination hint")
	}
}

func TestKakaoTemplateList_StatusFilter(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("status") != "APPROVED" {
			t.Errorf("expected status=APPROVED, got %s", r.URL.Query().Get("status"))
		}
		resp := types.KakaoTemplateListResponse{TemplateList: []types.KakaoTemplate{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "template", "list", "--status", "APPROVED"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKakaoTemplateList_Alias(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		resp := types.KakaoTemplateListResponse{TemplateList: []types.KakaoTemplate{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "tpl", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error with alias 'tpl': %v", err)
	}
}

// --- Template Get Tests ---

func TestKakaoTemplateGet_Success(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "FAKE_TPL_001") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		tpl := types.KakaoTemplate{
			TemplateID:   "FAKE_TPL_001",
			Name:         "주문 확인",
			Status:       "APPROVED",
			MessageType:  "BA",
			Content:      "#{고객명}님 주문이 완료되었습니다.",
			CategoryCode: "001001",
			ChannelID:    "FAKE_CH_001",
			AccountID:    "FAKE_ACCOUNT",
			DateCreated:  "2025-01-01T00:00:00.000Z",
			DateUpdated:  "2025-01-02T00:00:00.000Z",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tpl)
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "template", "get", "FAKE_TPL_001"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "FAKE_TPL_001") {
		t.Error("expected template ID")
	}
	if !strings.Contains(output, "주문 확인") {
		t.Error("expected template name")
	}
}

func TestKakaoTemplateGet_NotFound(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorCode":"TemplateNotFound","errorMessage":"템플릿을 찾을 수 없습니다."}`))
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "template", "get", "NONEXISTENT"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

// --- Template Create Tests ---

func TestKakaoTemplateCreate_Success(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]interface{}
		_ = json.Unmarshal(body, &parsed)
		if parsed["channelId"] != "FAKE_CH_001" {
			t.Errorf("expected channelId=FAKE_CH_001")
		}
		if parsed["name"] != "테스트" {
			t.Errorf("expected name=테스트")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"templateId":"FAKE_TPL_NEW","name":"테스트","status":"PENDING"}`))
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{
		"kakao", "template", "create",
		"--channel-id", "FAKE_CH_001",
		"--name", "테스트",
		"--content", "내용입니다",
		"--category-code", "001001",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKakaoTemplateCreate_XOR_BothProvided(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{
		"kakao", "template", "create",
		"--channel-id", "FAKE_CH_001",
		"--channel-group-id", "FAKE_GRP_001",
		"--name", "test",
		"--content", "c",
		"--category-code", "001",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when both channel-id and channel-group-id provided")
	}
	if !strings.Contains(err.Error(), "동시에 사용할 수 없습니다") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestKakaoTemplateCreate_XOR_NeitherProvided(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{
		"kakao", "template", "create",
		"--name", "test",
		"--content", "c",
		"--category-code", "001",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when neither channel-id nor channel-group-id provided")
	}
	if !strings.Contains(err.Error(), "--channel-id 또는 --channel-group-id") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestKakaoTemplateCreate_MissingName(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{
		"kakao", "template", "create",
		"--channel-id", "FAKE_CH_001",
		"--content", "c",
		"--category-code", "001",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "--name") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestKakaoTemplateCreate_MissingContent(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{
		"kakao", "template", "create",
		"--channel-id", "FAKE_CH_001",
		"--name", "n",
		"--category-code", "001",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing content")
	}
}

func TestKakaoTemplateCreate_InvalidButtonsJSON(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{
		"kakao", "template", "create",
		"--channel-id", "FAKE_CH_001",
		"--name", "n",
		"--content", "c",
		"--category-code", "001",
		"--buttons", "invalid json",
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid buttons JSON")
	}
	if !strings.Contains(err.Error(), "JSON 파싱 실패") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestKakaoTemplateCreate_WithAllFlags(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]interface{}
		_ = json.Unmarshal(body, &parsed)
		if parsed["messageType"] != "AD" {
			t.Errorf("expected messageType=AD, got %v", parsed["messageType"])
		}
		if parsed["emphasizeType"] != "TEXT" {
			t.Errorf("expected emphasizeType=TEXT, got %v", parsed["emphasizeType"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"templateId":"FAKE_TPL_NEW"}`))
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{
		"kakao", "template", "create",
		"--channel-group-id", "FAKE_GRP_001",
		"--name", "n",
		"--content", "c",
		"--category-code", "001",
		"--message-type", "AD",
		"--emphasize-type", "TEXT",
		"--emphasize-title", "제목",
		"--emphasize-subtitle", "부제목",
		"--header", "헤더",
		"--extra", "부가",
		"--ad", "광고",
		"--buttons", `[{"buttonType":"WL","buttonName":"btn","linkMo":"https://example.com"}]`,
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Template Update Tests ---

func TestKakaoTemplateUpdate_PartialUpdate(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]interface{}
		_ = json.Unmarshal(body, &parsed)
		if parsed["name"] != "새이름" {
			t.Errorf("expected name=새이름, got %v", parsed["name"])
		}
		// content should NOT be in body since it wasn't changed
		if _, ok := parsed["content"]; ok {
			t.Error("content should not be in body for partial update")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"templateId":"FAKE_TPL_001"}`))
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "template", "update", "FAKE_TPL_001", "--name", "새이름"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKakaoTemplateUpdate_APIError(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"errorCode":"FailedToUpdate","errorMessage":"수정 불가"}`))
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "template", "update", "FAKE_TPL_001", "--name", "n"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for 409 response")
	}
}

// --- Template Delete Tests ---

func TestKakaoTemplateDelete_WithYes(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "template", "delete", "FAKE_TPL_001", "--yes"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "삭제되었습니다") {
		t.Error("expected success message")
	}
}

func TestKakaoTemplateDelete_ConfirmYes(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	buf := captureBuf(t)
	stdinReader = bufio.NewReader(strings.NewReader("y\n"))
	t.Cleanup(func() { stdinReader = nil })

	rootCmd.SetArgs([]string{"kakao", "template", "delete", "FAKE_TPL_001"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "삭제되었습니다") {
		t.Error("expected success message after confirmation")
	}
}

func TestKakaoTemplateDelete_ConfirmNo(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not call API when user rejects")
	})

	buf := captureBuf(t)
	stdinReader = bufio.NewReader(strings.NewReader("n\n"))
	t.Cleanup(func() { stdinReader = nil })

	rootCmd.SetArgs([]string{"kakao", "template", "delete", "FAKE_TPL_001"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "취소되었습니다") {
		t.Error("expected cancellation message")
	}
}

func TestKakaoTemplateDelete_ConfirmEmpty(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not call API for empty input")
	})

	buf := captureBuf(t)
	stdinReader = bufio.NewReader(strings.NewReader("\n"))
	t.Cleanup(func() { stdinReader = nil })

	rootCmd.SetArgs([]string{"kakao", "template", "delete", "FAKE_TPL_001"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "취소되었습니다") {
		t.Error("expected cancellation for empty input")
	}
}

// --- Template Inspect Tests ---

func TestKakaoTemplateInspect_Success(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/inspection") {
			t.Errorf("expected /inspection in path, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"templateId":"FAKE_TPL_001","status":"INSPECTING"}`))
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "template", "inspect", "FAKE_TPL_001"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "검수가 요청되었습니다") {
		t.Error("expected inspection request message")
	}
}

func TestKakaoTemplateInspect_WithComment(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]interface{}
		_ = json.Unmarshal(body, &parsed)
		if parsed["comment"] != "검수 부탁드립니다" {
			t.Errorf("expected comment, got %v", parsed["comment"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"templateId":"FAKE_TPL_001","status":"INSPECTING"}`))
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "template", "inspect", "FAKE_TPL_001", "--comment", "검수 부탁드립니다"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Template Cancel Inspect Tests ---

func TestKakaoTemplateCancelInspect_Success(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/inspection/cancel") {
			t.Errorf("expected /inspection/cancel in path, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"templateId":"FAKE_TPL_001","status":"PENDING"}`))
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "template", "cancel-inspect", "FAKE_TPL_001"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "검수가 취소되었습니다") {
		t.Error("expected cancel inspection message")
	}
}

// --- Template Sendable Tests ---

func TestKakaoTemplateSendable_Success(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/sendable") {
			t.Errorf("expected /sendable in path, got %s", r.URL.Path)
		}
		templates := []types.KakaoTemplate{
			{TemplateID: "FAKE_TPL_001", Name: "발송 가능", Status: "APPROVED", MessageType: "BA", ChannelID: "FAKE_CH_001"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(templates)
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "template", "sendable"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "FAKE_TPL_001") {
		t.Error("expected template in sendable list")
	}
}

// --- Template Categories Tests ---

func TestKakaoTemplateCategories_Success(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "templates/categories") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		categories := []types.KakaoTemplateCategory{
			{Code: "001001", Name: "건강"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(categories)
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "template", "categories"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "001001") {
		t.Error("expected category code")
	}
}

// --- Template Release Dormant Tests ---

func TestKakaoTemplateReleaseDormant_Success(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		// Verify the intentional typo in the API path
		if !strings.Contains(r.URL.Path, "relese-dormant") {
			t.Errorf("expected 'relese-dormant' (typo) in path, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"templateId":"FAKE_TPL_001","status":"APPROVED"}`))
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "template", "release-dormant", "FAKE_TPL_001"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "휴면이 해제되었습니다") {
		t.Error("expected dormant release message")
	}
}

func TestKakaoTemplateReleaseDormant_APIError(t *testing.T) {
	setupKakaoTemplateTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"errorCode":"NotAnActiveTemplate","errorMessage":"활성화된 템플릿이 아닙니다."}`))
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "template", "release-dormant", "FAKE_TPL_001"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for 409")
	}
}
