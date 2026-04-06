// Package types defines SOLAPI API request/response structures.
// Types are based on solapi-go/v2 messages/types.go but defined independently.
package types

import "runtime"

// Message represents a single message in a send request.
type Message struct {
	To           string        `json:"to"`
	From         string        `json:"from,omitempty"`
	Text         string        `json:"text,omitempty"`
	Type         string        `json:"type,omitempty"`
	Subject      string        `json:"subject,omitempty"`
	ImageID      string        `json:"imageId,omitempty"`
	Country      string        `json:"country,omitempty"`
	KakaoOptions *KakaoOptions `json:"kakaoOptions,omitempty"`
	RCSOptions   *RCSOptions   `json:"rcsOptions,omitempty"`
	CustomFields map[string]string `json:"customFields,omitempty"`
}

// KakaoOptions holds Kakao channel messaging options.
type KakaoOptions struct {
	PfID       string            `json:"pfId,omitempty"`
	TemplateID string            `json:"templateId,omitempty"`
	SenderKey  string            `json:"senderKey,omitempty"`
	Title      string            `json:"title,omitempty"`
	AdFlag     *bool             `json:"adFlag,omitempty"`
	DisableSms *bool             `json:"disableSms,omitempty"`
	ImageID    string            `json:"imageId,omitempty"`
	Variables  map[string]string `json:"variables,omitempty"`
	Buttons    []KakaoButton     `json:"buttons,omitempty"`
	BMS        *KakaoBMSOptions  `json:"bms,omitempty"`
}

// KakaoButton represents a button in Kakao messages.
type KakaoButton struct {
	ButtonType string `json:"buttonType,omitempty"`
	ButtonName string `json:"buttonName,omitempty"`
	LinkMo     string `json:"linkMo,omitempty"`
	LinkPc     string `json:"linkPc,omitempty"`
	LinkAnd    string `json:"linkAnd,omitempty"`
	LinkIos    string `json:"linkIos,omitempty"`
}

// KakaoBMSOptions holds BMS-specific Kakao options.
type KakaoBMSOptions struct {
	Targeting      string `json:"targeting,omitempty"`      // "I", "M", "N"
	ChatBubbleType string `json:"chatBubbleType,omitempty"` // TEXT, IMAGE, WIDE, etc.
	Adult          *bool  `json:"adult,omitempty"`
}

// RCSOptions holds RCS messaging options.
type RCSOptions struct {
	BrandID    string            `json:"brandId,omitempty"`
	TemplateID string            `json:"templateId,omitempty"`
	CopyAllowed *bool            `json:"copyAllowed,omitempty"`
	Variables  map[string]string `json:"variables,omitempty"`
	MmsType    string            `json:"mmsType,omitempty"`
}

// Agent identifies the SDK/tool sending the message.
type Agent struct {
	SDKVersion string `json:"sdkVersion,omitempty"`
	OSPlatform string `json:"osPlatform,omitempty"`
	AppID      string `json:"appId,omitempty"`
}

// DefaultAgent returns an Agent populated with solactl info.
func DefaultAgent(version string) *Agent {
	return &Agent{
		SDKVersion: "solactl/" + version,
		OSPlatform: runtime.GOOS,
	}
}

// SendRequest is the request body for POST /messages/v4/send-many/detail.
type SendRequest struct {
	Messages        []Message `json:"messages"`
	ScheduledDate   string    `json:"scheduledDate,omitempty"`
	Agent           *Agent    `json:"agent,omitempty"`
	AllowDuplicates *bool     `json:"allowDuplicates,omitempty"`
	ShowMessageList *bool     `json:"showMessageList,omitempty"`
}

// SendResponse is the response from send-many/detail.
type SendResponse struct {
	GroupInfo         GroupInfo       `json:"groupInfo"`
	FailedMessageList []FailedMessage `json:"failedMessageList"`
	MessageList       []MessageResult `json:"messageList,omitempty"`
}

// GroupInfo contains summary information about the sent group.
type GroupInfo struct {
	Count     GroupCount `json:"count"`
	GroupID   string     `json:"groupId"`
	Status    string     `json:"status"`
	AccountID string     `json:"accountId"`
}

// GroupCount holds message count statistics.
type GroupCount struct {
	Total             int `json:"total"`
	SentTotal         int `json:"sentTotal"`
	SentFailed        int `json:"sentFailed"`
	SentSuccess       int `json:"sentSuccess"`
	SentPending       int `json:"sentPending"`
	RegisteredFailed  int `json:"registeredFailed"`
	RegisteredSuccess int `json:"registeredSuccess"`
}

// FailedMessage represents a message that failed to register.
type FailedMessage struct {
	To            string `json:"to"`
	From          string `json:"from"`
	Type          string `json:"type"`
	MessageID     string `json:"messageId"`
	StatusCode    string `json:"statusCode"`
	StatusMessage string `json:"statusMessage"`
	AccountID     string `json:"accountId"`
}

// MessageResult is a per-message result in send-many/detail response.
type MessageResult struct {
	MessageID     string `json:"messageId"`
	StatusCode    string `json:"statusCode"`
	StatusMessage string `json:"statusMessage,omitempty"`
}

// UploadFileRequest is the request for POST /storage/v1/files.
type UploadFileRequest struct {
	File string `json:"file"` // base64-encoded file content
	Type string `json:"type,omitempty"`
	Name string `json:"name,omitempty"`
}

// UploadFileResponse is the response from file upload.
type UploadFileResponse struct {
	FileID string `json:"fileId"`
	Name   string `json:"name"`
	URL    string `json:"url"`
}
