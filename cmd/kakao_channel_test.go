package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/solapi/solactl/pkg/client"
	"github.com/solapi/solactl/pkg/types"
)

// resetKakaoChannelFlags resets all kakao channel-related flags.
func resetKakaoChannelFlags() {
	resetFlags()
	kakaoChListFlagChannelID = ""
	kakaoChListFlagSearchID = ""
	kakaoChListFlagPhoneNumber = ""
	kakaoChListFlagCategoryCode = ""
	kakaoChListFlagIsMine = false
	kakaoChListFlagStartKey = ""
	kakaoChListFlagLimit = 20
	kakaoChgListFlagGroupID = ""
	kakaoChgListFlagName = ""
	kakaoChgListFlagStatus = ""
	kakaoChgListFlagIsMine = false
	kakaoChgListFlagStartKey = ""
	kakaoChgListFlagLimit = 20
}

// setupKakaoTest creates a test environment for kakao commands.
func setupKakaoTest(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("SOLACTL_API_KEY", "testkey")
	t.Setenv("SOLACTL_API_SECRET", "testsecret")

	resetKakaoChannelFlags()

	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	c := &client.Client{
		HTTPClient:      ts.Client(),
		APIKey:          "testkey",
		APISecret:       "testsecret",
		MaxRetries:      0,
		BaseDelay:       time.Millisecond,
		BaseURLOverride: ts.URL,
	}
	clientOverride = c
	t.Cleanup(func() {
		clientOverride = nil
		outWriter = nil
		errWriter = nil
		resetKakaoChannelFlags()
	})

	return ts
}

// --- Channel List Tests ---

func TestKakaoChannelList_Success(t *testing.T) {
	ts := setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/kakao/v2/channels") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		resp := types.KakaoChannelListResponse{
			ChannelList: []types.KakaoChannel{
				{
					ChannelID:   "FAKE_CH_001",
					SearchID:    "@fakeChannel",
					PhoneNumber: "01000000000",
					IsBrand:     false,
					DateCreated: "2025-01-01T00:00:00.000Z",
				},
				{
					ChannelID:   "FAKE_CH_002",
					SearchID:    "@fakeChannel2",
					PhoneNumber: "01011111111",
					IsBrand:     true,
					DateCreated: "2025-02-01T12:30:00.000Z",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	_ = ts

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "channel", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "FAKE_CH_001") {
		t.Error("expected FAKE_CH_001 in output")
	}
	if !strings.Contains(output, "@fakeChannel") {
		t.Error("expected @fakeChannel in output")
	}
	if !strings.Contains(output, "CHANNEL ID") {
		t.Error("expected table header CHANNEL ID")
	}
}

func TestKakaoChannelList_EmptyResult(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		resp := types.KakaoChannelListResponse{
			ChannelList: []types.KakaoChannel{},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "channel", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "CHANNEL ID") {
		t.Error("expected table header even for empty list")
	}
}

func TestKakaoChannelList_Pagination_NextKeyPresent(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		resp := types.KakaoChannelListResponse{
			ChannelList: []types.KakaoChannel{
				{ChannelID: "FAKE_CH_001", SearchID: "@ch1", PhoneNumber: "01000000000", DateCreated: "2025-01-01T00:00:00Z"},
			},
			NextKey: "next_page_key_abc",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	buf := captureBuf(t)
	errBuf := captureErrBuf(t)
	rootCmd.SetArgs([]string{"kakao", "channel", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = buf.String()
	errOutput := errBuf.String()
	if !strings.Contains(errOutput, "다음 페이지") {
		t.Error("expected next page hint in stderr")
	}
	if !strings.Contains(errOutput, "next_page_key_abc") {
		t.Error("expected nextKey value in pagination hint")
	}
}

func TestKakaoChannelList_Pagination_NoNextKey(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		resp := types.KakaoChannelListResponse{
			ChannelList: []types.KakaoChannel{
				{ChannelID: "FAKE_CH_001", SearchID: "@ch1", PhoneNumber: "01000000000", DateCreated: "2025-01-01T00:00:00Z"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	captureBuf(t)
	errBuf := captureErrBuf(t)
	rootCmd.SetArgs([]string{"kakao", "channel", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(errBuf.String(), "다음 페이지") {
		t.Error("should not show pagination hint when no nextKey")
	}
}

func TestKakaoChannelList_JSONMode(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		resp := types.KakaoChannelListResponse{
			ChannelList: []types.KakaoChannel{
				{ChannelID: "FAKE_CH_001", SearchID: "@ch1", PhoneNumber: "01000000000", DateCreated: "2025-01-01T00:00:00Z"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "channel", "list", "--json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &parsed); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}
	if _, ok := parsed["channelList"]; !ok {
		t.Error("expected channelList key in JSON output")
	}
}

func TestKakaoChannelList_FilterFlagsPassedAsQueryParams(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("channelId") != "FAKE_CH_FILTER" {
			t.Errorf("expected channelId=FAKE_CH_FILTER, got %s", q.Get("channelId"))
		}
		if q.Get("searchId") != "@test" {
			t.Errorf("expected searchId=@test, got %s", q.Get("searchId"))
		}
		if q.Get("limit") != "5" {
			t.Errorf("expected limit=5, got %s", q.Get("limit"))
		}
		if q.Get("startKey") != "sk123" {
			t.Errorf("expected startKey=sk123, got %s", q.Get("startKey"))
		}
		resp := types.KakaoChannelListResponse{ChannelList: []types.KakaoChannel{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{
		"kakao", "channel", "list",
		"--channel-id", "FAKE_CH_FILTER",
		"--search-id", "@test",
		"--limit", "5",
		"--start-key", "sk123",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKakaoChannelList_APIError(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errorCode":"InternalError","errorMessage":"서버 오류"}`))
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "channel", "list"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "채널 목록 조회 실패") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestKakaoChannelList_MalformedJSONResponse(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{invalid json`))
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "channel", "list"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for malformed JSON response")
	}
	if !strings.Contains(err.Error(), "파싱 실패") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// --- Channel Get Tests ---

func TestKakaoChannelGet_Success(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "FAKE_CH_001") {
			t.Errorf("expected path to contain FAKE_CH_001, got %s", r.URL.Path)
		}
		ch := types.KakaoChannel{
			ChannelID:   "FAKE_CH_001",
			AccountID:   "FAKE_ACCOUNT",
			SearchID:    "@fakeChannel",
			PhoneNumber: "01000000000",
			IsBrand:     true,
			DateCreated: "2025-01-01T00:00:00.000Z",
			DateUpdated: "2025-01-02T00:00:00.000Z",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ch)
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "channel", "get", "FAKE_CH_001"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "FAKE_CH_001") {
		t.Error("expected FAKE_CH_001 in output")
	}
	if !strings.Contains(output, "@fakeChannel") {
		t.Error("expected @fakeChannel in output")
	}
	if !strings.Contains(output, "Yes") {
		t.Error("expected Brand=Yes in output")
	}
}

func TestKakaoChannelGet_NotFound(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorCode":"ChannelNotFound","errorMessage":"카카오톡 채널를 찾을 수 없습니다."}`))
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "channel", "get", "NONEXISTENT"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "채널 조회 실패") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestKakaoChannelGet_JSONMode(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		ch := types.KakaoChannel{
			ChannelID:   "FAKE_CH_001",
			SearchID:    "@fakeChannel",
			PhoneNumber: "01000000000",
			DateCreated: "2025-01-01T00:00:00.000Z",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ch)
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "channel", "get", "FAKE_CH_001", "--json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &parsed); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}
}

func TestKakaoChannelGet_MissingArg(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called when arg is missing")
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "channel", "get"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing channelId arg")
	}
}

// --- Channel Categories Tests ---

func TestKakaoChannelCategories_Success(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/categories") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		categories := []types.KakaoTemplateCategory{
			{Code: "001001", Name: "건강"},
			{Code: "002001", Name: "교육"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(categories)
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "channel", "categories"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "001001") {
		t.Error("expected category code 001001")
	}
	if !strings.Contains(output, "건강") {
		t.Error("expected category name 건강")
	}
	if !strings.Contains(output, "CODE") {
		t.Error("expected table header CODE")
	}
}

func TestKakaoChannelCategories_APIError(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errorCode":"InternalError","errorMessage":"서버 오류"}`))
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "channel", "categories"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// --- Channel Group List Tests ---

func TestKakaoChannelGroupList_Success(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/kakao/v2/channel-groups") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		resp := types.KakaoChannelGroupListResponse{
			ChannelGroupList: []types.KakaoChannelGroup{
				{
					ChannelGroupID: "FAKE_GRP_001",
					Name:           "테스트 그룹",
					Status:         "APPROVED",
					Type:           "PRIVATE",
					ChannelIDs:     []string{"FAKE_CH_001", "FAKE_CH_002"},
					IsBrand:        false,
					DateCreated:    "2025-01-01T00:00:00.000Z",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "channel-group", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "FAKE_GRP_001") {
		t.Error("expected FAKE_GRP_001 in output")
	}
	if !strings.Contains(output, "테스트 그룹") {
		t.Error("expected group name in output")
	}
	if !strings.Contains(output, "2") {
		t.Error("expected channel count 2 in output")
	}
}

func TestKakaoChannelGroupList_Pagination(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		resp := types.KakaoChannelGroupListResponse{
			ChannelGroupList: []types.KakaoChannelGroup{
				{ChannelGroupID: "FAKE_GRP_001", Name: "G1", Status: "APPROVED", ChannelIDs: []string{}, DateCreated: "2025-01-01T00:00:00Z"},
			},
			NextKey: "grp_next_key",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	captureBuf(t)
	errBuf := captureErrBuf(t)
	rootCmd.SetArgs([]string{"kakao", "channel-group", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	errOutput := errBuf.String()
	if !strings.Contains(errOutput, "다음 페이지") {
		t.Error("expected pagination hint in stderr")
	}
	if !strings.Contains(errOutput, "grp_next_key") {
		t.Error("expected nextKey in pagination hint")
	}
}

func TestKakaoChannelGroupList_FilterFlags(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("status") != "APPROVED" {
			t.Errorf("expected status=APPROVED, got %s", q.Get("status"))
		}
		if q.Get("name") != "testGroup" {
			t.Errorf("expected name=testGroup, got %s", q.Get("name"))
		}
		if q.Get("limit") != "10" {
			t.Errorf("expected limit=10, got %s", q.Get("limit"))
		}
		resp := types.KakaoChannelGroupListResponse{ChannelGroupList: []types.KakaoChannelGroup{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{
		"kakao", "channel-group", "list",
		"--status", "APPROVED",
		"--name", "testGroup",
		"--limit", "10",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKakaoChannelGroupList_APIError(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errorCode":"InternalError","errorMessage":"서버 오류"}`))
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "channel-group", "list"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// --- Channel Group Get Tests ---

func TestKakaoChannelGroupGet_Success(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "FAKE_GRP_001") {
			t.Errorf("expected path to contain FAKE_GRP_001, got %s", r.URL.Path)
		}
		group := types.KakaoChannelGroup{
			ChannelGroupID: "FAKE_GRP_001",
			AccountID:      "FAKE_ACCOUNT",
			Name:           "테스트 그룹",
			ChannelIDs:     []string{"FAKE_CH_001", "FAKE_CH_002"},
			Type:           "PRIVATE",
			Status:         "APPROVED",
			IsBrand:        false,
			DateCreated:    "2025-01-01T00:00:00.000Z",
			DateUpdated:    "2025-01-02T00:00:00.000Z",
			Channels: []types.KakaoChannel{
				{ChannelID: "FAKE_CH_001", SearchID: "@ch1"},
				{ChannelID: "FAKE_CH_002", SearchID: "@ch2", IsBrand: true},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(group)
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "channel-group", "get", "FAKE_GRP_001"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "FAKE_GRP_001") {
		t.Error("expected FAKE_GRP_001 in output")
	}
	if !strings.Contains(output, "테스트 그룹") {
		t.Error("expected group name in output")
	}
	if !strings.Contains(output, "소속 채널") {
		t.Error("expected channel list section")
	}
	if !strings.Contains(output, "FAKE_CH_001") {
		t.Error("expected FAKE_CH_001 in channel list")
	}
	if !strings.Contains(output, "@ch2") {
		t.Error("expected @ch2 in channel list")
	}
}

func TestKakaoChannelGroupGet_NoChannels(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		group := types.KakaoChannelGroup{
			ChannelGroupID: "FAKE_GRP_001",
			Name:           "빈 그룹",
			ChannelIDs:     []string{},
			Status:         "PENDING",
			Type:           "PRIVATE",
			DateCreated:    "2025-01-01T00:00:00.000Z",
			DateUpdated:    "2025-01-01T00:00:00.000Z",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(group)
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "channel-group", "get", "FAKE_GRP_001"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "소속 채널") {
		t.Error("should not show channel list section when no channels")
	}
}

func TestKakaoChannelGroupGet_NotFound(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorCode":"ChannelGroupNotFound","errorMessage":"카카오톡 채널 그룹을 찾을 수 없습니다."}`))
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "channel-group", "get", "NONEXISTENT"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestKakaoChannelGroupGet_JSONMode(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		group := types.KakaoChannelGroup{
			ChannelGroupID: "FAKE_GRP_001",
			Name:           "그룹",
			ChannelIDs:     []string{},
			Status:         "APPROVED",
			Type:           "PRIVATE",
			DateCreated:    "2025-01-01T00:00:00.000Z",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(group)
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "channel-group", "get", "FAKE_GRP_001", "--json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}
}

func TestKakaoChannelGroupGet_MissingArg(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called when arg is missing")
	})

	captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "channel-group", "get"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing channelGroupId arg")
	}
}

// --- Alias Tests ---

func TestKakaoChannelAlias(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		resp := types.KakaoChannelListResponse{ChannelList: []types.KakaoChannel{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "ch", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error with alias 'ch': %v", err)
	}

	if !strings.Contains(buf.String(), "CHANNEL ID") {
		t.Error("expected table output with 'ch' alias")
	}
}

func TestKakaoChannelGroupAlias(t *testing.T) {
	setupKakaoTest(t, func(w http.ResponseWriter, r *http.Request) {
		resp := types.KakaoChannelGroupListResponse{ChannelGroupList: []types.KakaoChannelGroup{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	buf := captureBuf(t)
	rootCmd.SetArgs([]string{"kakao", "chg", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error with alias 'chg': %v", err)
	}

	if !strings.Contains(buf.String(), "GROUP ID") {
		t.Error("expected table output with 'chg' alias")
	}
}

// captureErrBuf is a test helper defined in send_test.go.
// If not available, define here. Since all files are in the same package, we check:
var _ = captureErrBuf // ensure it exists from send_test.go
var _ = captureBuf    // ensure it exists from send_test.go
