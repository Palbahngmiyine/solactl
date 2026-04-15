package types

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDisplayDate_TruncatesLongDates(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"ISO 8601 full", "2025-01-01T00:00:00.000Z", "2025-01-01T00:00"},
		{"ISO 8601 short", "2025-01-01T00:00", "2025-01-01T00:00"},
		{"date only", "2025-01-01", "2025-01-01"},
		{"empty", "", "-"},
		{"short string", "abc", "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DisplayDate(tt.input)
			if got != tt.expected {
				t.Errorf("DisplayDate(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestDisplayStatus_EmptyReturnsDash(t *testing.T) {
	if got := DisplayStatus(""); got != "-" {
		t.Errorf("DisplayStatus(\"\") = %q, want \"-\"", got)
	}
	if got := DisplayStatus("APPROVED"); got != "APPROVED" {
		t.Errorf("DisplayStatus(\"APPROVED\") = %q, want \"APPROVED\"", got)
	}
}

func TestDisplayBool(t *testing.T) {
	if got := DisplayBool(true); got != "Yes" {
		t.Errorf("DisplayBool(true) = %q, want \"Yes\"", got)
	}
	if got := DisplayBool(false); got != "No" {
		t.Errorf("DisplayBool(false) = %q, want \"No\"", got)
	}
}

func TestKakaoChannel_JSONRoundTrip(t *testing.T) {
	ch := KakaoChannel{
		ChannelID:   "FAKE_CH_001",
		AccountID:   "FAKE_ACCOUNT",
		SearchID:    "@fakeChannel",
		PhoneNumber: "01000000000",
		IsBrand:     true,
		DateCreated: "2025-01-01T00:00:00.000Z",
		DateUpdated: "2025-01-02T00:00:00.000Z",
	}

	data, err := json.Marshal(ch)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded KakaoChannel
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.ChannelID != ch.ChannelID {
		t.Errorf("ChannelID mismatch: %s != %s", decoded.ChannelID, ch.ChannelID)
	}
	if decoded.IsBrand != ch.IsBrand {
		t.Errorf("IsBrand mismatch: %v != %v", decoded.IsBrand, ch.IsBrand)
	}
}

func TestKakaoChannel_SenderKeysNotExposed(t *testing.T) {
	// Verify that senderKeys from API response is silently ignored
	rawJSON := `{"channelId":"FAKE_CH_001","searchId":"@ch","phoneNumber":"01000000000","senderKeys":[{"key":"secret","service":"biz"}],"dateCreated":"2025-01-01","dateUpdated":"2025-01-01"}`
	var ch KakaoChannel
	if err := json.Unmarshal([]byte(rawJSON), &ch); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	// senderKeys should not be accessible since the struct has no such field
	reMarshaled, _ := json.Marshal(ch)
	reStr := string(reMarshaled)
	if strings.Contains(reStr,"senderKeys") || strings.Contains(reStr, "secret") {
		t.Error("senderKeys should not be exposed in re-marshaled output")
	}
}

func TestKakaoChannelGroup_DisplayChannelCount(t *testing.T) {
	g := KakaoChannelGroup{ChannelIDs: []string{"a", "b", "c"}}
	if got := g.DisplayChannelCount(); got != "3" {
		t.Errorf("DisplayChannelCount() = %q, want \"3\"", got)
	}

	g2 := KakaoChannelGroup{ChannelIDs: []string{}}
	if got := g2.DisplayChannelCount(); got != "0" {
		t.Errorf("DisplayChannelCount() = %q, want \"0\"", got)
	}
}

func TestKakaoChannelGroup_GroupKeysNotExposed(t *testing.T) {
	rawJSON := `{"channelGroupId":"FAKE_GRP_001","name":"g","channelIds":[],"groupKeys":[{"key":"secret","service":"biz"}],"status":"APPROVED","type":"PRIVATE","dateCreated":"2025-01-01","dateUpdated":"2025-01-01"}`
	var g KakaoChannelGroup
	if err := json.Unmarshal([]byte(rawJSON), &g); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	reMarshaled, _ := json.Marshal(g)
	reStr := string(reMarshaled)
	if strings.Contains(reStr,"groupKeys") || strings.Contains(reStr, "secret") {
		t.Error("groupKeys should not be exposed in re-marshaled output")
	}
}

func TestKakaoTemplate_DisplayMessageType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"BA", "BA"},
		{"EX", "EX"},
		{"AD", "AD"},
		{"MI", "MI"},
		{"", "-"},
		{"UNKNOWN", "UNKNOWN"},
	}
	for _, tt := range tests {
		tpl := KakaoTemplate{MessageType: tt.input}
		got := tpl.DisplayMessageType()
		if got != tt.expected {
			t.Errorf("DisplayMessageType(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestKakaoTemplate_DisplayChannelRef(t *testing.T) {
	tests := []struct {
		name           string
		channelID      string
		channelGroupID string
		expected       string
	}{
		{"channel", "CH001", "", "CH001"},
		{"group", "", "GRP001", "(grp)GRP001"},
		{"neither", "", "", "-"},
		{"both prefers channel", "CH001", "GRP001", "CH001"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tpl := KakaoTemplate{ChannelID: tt.channelID, ChannelGroupID: tt.channelGroupID}
			got := tpl.DisplayChannelRef()
			if got != tt.expected {
				t.Errorf("DisplayChannelRef() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestKakaoTemplate_JSONRoundTrip(t *testing.T) {
	tpl := KakaoTemplate{
		TemplateID:   "FAKE_TPL_001",
		Name:         "테스트 템플릿",
		Content:      "안녕하세요 #{고객명}님",
		CategoryCode: "001001",
		Status:       "APPROVED",
		MessageType:  "BA",
		DateCreated:  "2025-01-01T00:00:00Z",
		DateUpdated:  "2025-01-01T00:00:00Z",
	}

	data, err := json.Marshal(tpl)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded KakaoTemplate
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.TemplateID != tpl.TemplateID {
		t.Errorf("TemplateID mismatch: %s != %s", decoded.TemplateID, tpl.TemplateID)
	}
}

func TestKakaoBrandTemplate_JSONRoundTrip(t *testing.T) {
	bt := KakaoBrandTemplate{
		BrandTemplateID: "FAKE_BT_001",
		Name:            "브랜드 템플릿",
		ChatBubbleType:  "TEXT",
		Content:         "테스트 내용",
		Status:          "ACTIVE",
		DateCreated:     "2025-01-01T00:00:00Z",
		DateUpdated:     "2025-01-01T00:00:00Z",
	}

	data, err := json.Marshal(bt)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded KakaoBrandTemplate
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.BrandTemplateID != bt.BrandTemplateID {
		t.Errorf("BrandTemplateID mismatch: %s != %s", decoded.BrandTemplateID, bt.BrandTemplateID)
	}
	if decoded.ChatBubbleType != bt.ChatBubbleType {
		t.Errorf("ChatBubbleType mismatch: %s != %s", decoded.ChatBubbleType, bt.ChatBubbleType)
	}
}

func TestKakaoChannelListResponse_JSONParsing(t *testing.T) {
	raw := `{"channelList":[{"channelId":"FAKE_CH","searchId":"@ch","phoneNumber":"010","dateCreated":"2025-01-01","dateUpdated":"2025-01-01"}],"nextKey":"nk","limit":20}`
	var resp KakaoChannelListResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(resp.ChannelList) != 1 {
		t.Errorf("expected 1 channel, got %d", len(resp.ChannelList))
	}
	if resp.NextKey != "nk" {
		t.Errorf("expected nextKey=nk, got %s", resp.NextKey)
	}
}
