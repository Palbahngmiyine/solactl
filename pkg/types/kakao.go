package types

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// --- Display helpers ---

// DisplayDate truncates an ISO 8601 date string to "YYYY-MM-DDTHH:MM" (16 chars).
func DisplayDate(s string) string {
	if len(s) > 16 {
		return s[:16]
	}
	if s == "" {
		return "-"
	}
	return s
}

// DisplayStatus returns the status string or "-" if empty.
func DisplayStatus(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// DisplayBool returns "Yes" or "No" for a bool value.
func DisplayBool(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

// FormatThousands renders an int with thousand separators. Negative numbers preserve '-'.
func FormatThousands(n int) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		if neg {
			return "-" + s
		}
		return s
	}
	head := len(s) % 3
	var b strings.Builder
	b.Grow(len(s) + len(s)/3 + 1)
	if neg {
		b.WriteByte('-')
	}
	if head > 0 {
		b.WriteString(s[:head])
		if len(s) > head {
			b.WriteByte(',')
		}
	}
	for i := head; i < len(s); i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < len(s) {
			b.WriteByte(',')
		}
	}
	return b.String()
}

// --- Channel types ---

// KakaoChannel represents a Kakao business channel (v2 API response).
// Note: senderKeys field intentionally omitted for security.
type KakaoChannel struct {
	ChannelID        string   `json:"channelId"`
	AccountID        string   `json:"accountId"`
	SearchID         string   `json:"searchId"`
	PhoneNumber      string   `json:"phoneNumber"`
	ChannelName      string   `json:"channelName,omitempty"`
	SharedAccountIDs []string `json:"sharedAccountIds,omitempty"`
	IsBrand          bool     `json:"isBrand"`
	DateCreated      string   `json:"dateCreated"`
	DateUpdated      string   `json:"dateUpdated"`
}

// KakaoChannelListResponse is the response from GET /kakao/v2/channels.
type KakaoChannelListResponse struct {
	ChannelList []KakaoChannel `json:"channelList"`
	StartKey    string         `json:"startKey,omitempty"`
	NextKey     string         `json:"nextKey,omitempty"`
	Limit       int            `json:"limit,omitempty"`
}

// --- Channel Group types ---

// KakaoChannelGroup represents a Kakao channel group (v2 API response).
// Note: groupKeys field intentionally omitted for security.
type KakaoChannelGroup struct {
	ChannelGroupID  string         `json:"channelGroupId"`
	AccountID       string         `json:"accountId"`
	Name            string         `json:"name"`
	ChannelIDs      []string       `json:"channelIds"`
	Type            string         `json:"type"`
	Status          string         `json:"status"`
	ReasonCreated   string         `json:"reasonCreated,omitempty"`
	ReasonInspected string         `json:"reasonInspected,omitempty"`
	IsBrand         bool           `json:"isBrand"`
	DateCreated     string         `json:"dateCreated"`
	DateUpdated     string         `json:"dateUpdated"`
	Channels        []KakaoChannel `json:"channels,omitempty"`
}

// DisplayChannelCount returns a string representation of the channel count.
func (g *KakaoChannelGroup) DisplayChannelCount() string {
	return strconv.Itoa(len(g.ChannelIDs))
}

// KakaoChannelGroupListResponse is the response from GET /kakao/v2/channel-groups.
type KakaoChannelGroupListResponse struct {
	ChannelGroupList []KakaoChannelGroup `json:"channelGroupList"`
	StartKey         string              `json:"startKey,omitempty"`
	NextKey          string              `json:"nextKey,omitempty"`
	Limit            int                 `json:"limit,omitempty"`
}

// --- Alimtalk Template types ---

// KakaoTemplate represents a Kakao alimtalk template (v2 API response).
type KakaoTemplate struct {
	TemplateID          string          `json:"templateId"`
	ChannelID           string          `json:"channelId,omitempty"`
	ChannelGroupID      string          `json:"channelGroupId,omitempty"`
	AssignType          string          `json:"assignType,omitempty"`
	Name                string          `json:"name"`
	Content             string          `json:"content"`
	CategoryCode        string          `json:"categoryCode"`
	AccountID           string          `json:"accountId"`
	Status              string          `json:"status"`
	Code                *string         `json:"code"`
	Comments            json.RawMessage `json:"comments,omitempty"`
	Commentable         bool            `json:"commentable"`
	IsHidden            bool            `json:"isHidden"`
	Buttons             []KakaoButton   `json:"buttons,omitempty"`
	QuickReplies        json.RawMessage `json:"quickReplies,omitempty"`
	MessageType         string          `json:"messageType,omitempty"`
	EmphasizeType       string          `json:"emphasizeType,omitempty"`
	Extra               *string         `json:"extra"`
	Ad                  *string         `json:"ad"`
	EmphasizeTitle      *string         `json:"emphasizeTitle"`
	EmphasizeSubtitle   *string         `json:"emphasizeSubtitle"`
	Header              *string         `json:"header"`
	Highlight           json.RawMessage `json:"highlight,omitempty"`
	Item                json.RawMessage `json:"item,omitempty"`
	SecurityFlag        bool            `json:"securityFlag"`
	ImageID             *string         `json:"imageId"`
	Variables           json.RawMessage `json:"variables,omitempty"`
	Codes               json.RawMessage `json:"codes,omitempty"`
	Replacements        json.RawMessage `json:"replacements,omitempty"`
	DisableReplacements *bool           `json:"disableReplacements"`
	DateCreated         string          `json:"dateCreated"`
	DateUpdated         string          `json:"dateUpdated"`
}

// DisplayMessageType returns a human-friendly message type label.
func (t *KakaoTemplate) DisplayMessageType() string {
	switch t.MessageType {
	case "BA":
		return "BA"
	case "EX":
		return "EX"
	case "AD":
		return "AD"
	case "MI":
		return "MI"
	default:
		return DisplayStatus(t.MessageType)
	}
}

// DisplayChannelRef returns the channel or channel group ID for display.
func (t *KakaoTemplate) DisplayChannelRef() string {
	if t.ChannelID != "" {
		return t.ChannelID
	}
	if t.ChannelGroupID != "" {
		return fmt.Sprintf("(grp)%s", t.ChannelGroupID)
	}
	return "-"
}

// KakaoTemplateListResponse is the response from GET /kakao/v2/templates.
type KakaoTemplateListResponse struct {
	TemplateList []KakaoTemplate `json:"templateList"`
	StartKey     string          `json:"startKey,omitempty"`
	NextKey      string          `json:"nextKey,omitempty"`
	Limit        int             `json:"limit,omitempty"`
}

// KakaoTemplateCategory represents a template or channel category.
type KakaoTemplateCategory struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// --- Brand Template types ---

// KakaoBrandTemplate represents a Kakao brand template (v2 API response).
// Note: buttons field uses json.RawMessage because brand template buttons
// have different field names (name, linkType, linkMobile) than alimtalk buttons
// (buttonName, buttonType, linkMo).
type KakaoBrandTemplate struct {
	BrandTemplateID     string          `json:"brandTemplateId"`
	Name                string          `json:"name"`
	AssignType          string          `json:"assignType,omitempty"`
	AccountID           string          `json:"accountId"`
	Adult               bool            `json:"adult"`
	ChatBubbleType      string          `json:"chatBubbleType"`
	Content             string          `json:"content,omitempty"`
	ImageID             string          `json:"imageId,omitempty"`
	ImageLink           string          `json:"imageLink,omitempty"`
	Header              string          `json:"header,omitempty"`
	AdditionalContent   string          `json:"additionalContent,omitempty"`
	Carousel            json.RawMessage `json:"carousel,omitempty"`
	MainWideItem        json.RawMessage `json:"mainWideItem,omitempty"`
	SubWideItemList     json.RawMessage `json:"subWideItemList,omitempty"`
	Video               json.RawMessage `json:"video,omitempty"`
	Commerce            json.RawMessage `json:"commerce,omitempty"`
	Buttons             json.RawMessage `json:"buttons,omitempty"`
	Coupon              json.RawMessage `json:"coupon,omitempty"`
	Status              string          `json:"status"`
	Code                *string         `json:"code"`
	Codes               json.RawMessage `json:"codes,omitempty"`
	Variables           json.RawMessage `json:"variables,omitempty"`
	AllowCopy           bool            `json:"allowCopy"`
	Replacements        json.RawMessage `json:"replacements,omitempty"`
	DisableReplacements *bool           `json:"disableReplacements"`
	DateCreated         string          `json:"dateCreated"`
	DateUpdated         string          `json:"dateUpdated"`
}

// KakaoBrandTemplateListResponse is the response from GET /kakao/v2/brand-templates.
type KakaoBrandTemplateListResponse struct {
	BrandTemplateList []KakaoBrandTemplate `json:"brandTemplateList"`
	StartKey          string               `json:"startKey,omitempty"`
	NextKey           string               `json:"nextKey,omitempty"`
	Limit             int                  `json:"limit,omitempty"`
}
