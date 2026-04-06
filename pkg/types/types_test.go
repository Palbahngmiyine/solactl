package types

import (
	"encoding/json"
	"runtime"
	"testing"
)

func TestDefaultAgent(t *testing.T) {
	a := DefaultAgent("1.0.0")
	if a.SDKVersion != "solactl/1.0.0" {
		t.Errorf("SDKVersion: got %q", a.SDKVersion)
	}
	if a.OSPlatform != runtime.GOOS {
		t.Errorf("OSPlatform: got %q, want %q", a.OSPlatform, runtime.GOOS)
	}
}

func TestSendRequest_JSON(t *testing.T) {
	show := true
	req := SendRequest{
		Messages: []Message{
			{To: "01012345678", From: "029999999", Text: "hello"},
		},
		Agent:           DefaultAgent("1.0.0"),
		ShowMessageList: &show,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	msgs, ok := parsed["messages"].([]any)
	if !ok || len(msgs) != 1 {
		t.Fatalf("messages: %v", parsed["messages"])
	}

	if parsed["showMessageList"] != true {
		t.Errorf("showMessageList: %v", parsed["showMessageList"])
	}
}

func TestMessage_OmitsEmptyFields(t *testing.T) {
	msg := Message{To: "01012345678"}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)

	omitFields := []string{"from", "text", "type", "subject", "imageId", "kakaoOptions", "rcsOptions", "customFields"}
	for _, f := range omitFields {
		if json.Valid(data) {
			var m map[string]any
			_ = json.Unmarshal(data, &m)
			if _, exists := m[f]; exists {
				t.Errorf("field %q should be omitted when empty, got: %s", f, s)
			}
		}
	}
}

func TestMessage_WithKakaoOptions(t *testing.T) {
	msg := Message{
		To:   "01012345678",
		From: "029999999",
		KakaoOptions: &KakaoOptions{
			PfID:       "KA01PF123",
			TemplateID: "KA01TP456",
			Variables:  map[string]string{"#{이름}": "홍길동"},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]any
	_ = json.Unmarshal(data, &parsed)
	kakao, ok := parsed["kakaoOptions"].(map[string]any)
	if !ok {
		t.Fatal("kakaoOptions missing")
	}
	if kakao["pfId"] != "KA01PF123" {
		t.Errorf("pfId: %v", kakao["pfId"])
	}
}

func TestMessage_WithRCSOptions(t *testing.T) {
	msg := Message{
		To:   "01012345678",
		Text: "RCS test",
		RCSOptions: &RCSOptions{
			BrandID: "BRAND123",
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]any
	_ = json.Unmarshal(data, &parsed)
	rcs, ok := parsed["rcsOptions"].(map[string]any)
	if !ok {
		t.Fatal("rcsOptions missing")
	}
	if rcs["brandId"] != "BRAND123" {
		t.Errorf("brandId: %v", rcs["brandId"])
	}
}

func TestSendResponse_Unmarshal(t *testing.T) {
	raw := `{
		"groupInfo": {
			"count": {"total": 1, "registeredSuccess": 1},
			"groupId": "G4V123",
			"status": "SENDING",
			"accountId": "ACC123"
		},
		"failedMessageList": [],
		"messageList": [{"messageId": "M4V123", "statusCode": "2000"}]
	}`

	var resp SendResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.GroupInfo.GroupID != "G4V123" {
		t.Errorf("groupId: %s", resp.GroupInfo.GroupID)
	}
	if resp.GroupInfo.Count.Total != 1 {
		t.Errorf("total: %d", resp.GroupInfo.Count.Total)
	}
	if len(resp.MessageList) != 1 {
		t.Fatalf("messageList len: %d", len(resp.MessageList))
	}
	if resp.MessageList[0].StatusCode != "2000" {
		t.Errorf("statusCode: %s", resp.MessageList[0].StatusCode)
	}
}

func TestUploadFileRequest_JSON(t *testing.T) {
	req := UploadFileRequest{
		File: "base64data==",
		Type: "MMS",
		Name: "test.jpg",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var parsed map[string]any
	_ = json.Unmarshal(data, &parsed)
	if parsed["file"] != "base64data==" {
		t.Errorf("file: %v", parsed["file"])
	}
}

// SenderID tests

func TestSenderID_DisplayStatus(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"ACTIVE", "ACTIVE"},
		{"PENDING", "PENDING"},
		{"", "-"},
	}
	for _, tt := range tests {
		s := &SenderID{Status: tt.status}
		if got := s.DisplayStatus(); got != tt.want {
			t.Errorf("DisplayStatus(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestSenderID_DisplayMethod(t *testing.T) {
	tests := []struct {
		method string
		want   string
	}{
		{"SELF-CERT", "SELF-CERT"},
		{"", "-"},
	}
	for _, tt := range tests {
		s := &SenderID{Method: tt.method}
		if got := s.DisplayMethod(); got != tt.want {
			t.Errorf("DisplayMethod(%q) = %q, want %q", tt.method, got, tt.want)
		}
	}
}

func TestSenderID_DisplayExpireAt(t *testing.T) {
	tests := []struct {
		expireAt string
		want     string
	}{
		{"2026-10-06T00:00:00Z", "2026-10-06"},
		{"2026-10-06", "2026-10-06"},
		{"short", "short"},
		{"", "-"},
	}
	for _, tt := range tests {
		s := &SenderID{ExpireAt: tt.expireAt}
		if got := s.DisplayExpireAt(); got != tt.want {
			t.Errorf("DisplayExpireAt(%q) = %q, want %q", tt.expireAt, got, tt.want)
		}
	}
}

func TestSenderIDInfo_Unmarshal(t *testing.T) {
	raw := `{
		"accountId": "ACC123",
		"limit": 5,
		"senderIds": [
			{"phoneNumber": "029999999", "status": "ACTIVE", "method": "SELF-CERT", "expireAt": "2026-10-06T00:00:00Z"},
			{"phoneNumber": "01012345678", "status": "PENDING"}
		]
	}`

	var info SenderIDInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if info.AccountID != "ACC123" {
		t.Errorf("accountId: %s", info.AccountID)
	}
	if len(info.SenderIDs) != 2 {
		t.Fatalf("senderIds len: %d", len(info.SenderIDs))
	}
	if info.SenderIDs[0].PhoneNumber != "029999999" {
		t.Errorf("first phone: %s", info.SenderIDs[0].PhoneNumber)
	}
}

func FuzzMessageJSON(f *testing.F) {
	f.Add(`{"to":"010","from":"029","text":"hi"}`)
	f.Add(`{}`)
	f.Add(`{"to":"","kakaoOptions":{"pfId":"test"}}`)
	f.Fuzz(func(t *testing.T, s string) {
		var m Message
		_ = json.Unmarshal([]byte(s), &m)
		// Re-marshal should not panic
		_, _ = json.Marshal(m)
	})
}
