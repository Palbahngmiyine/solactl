package types

// SenderID represents a single sender ID entry.
type SenderID struct {
	PhoneNumber string `json:"phoneNumber"`
	Status      string `json:"status"`
	Method      string `json:"method,omitempty"`
	ExpireAt    string `json:"expireAt,omitempty"`
	HandleKey   string `json:"handleKey,omitempty"`
	DateCreated string `json:"dateCreated,omitempty"`
	DateUpdated string `json:"dateUpdated,omitempty"`
}

// SenderIDInfo is the full sender ID document for an account.
type SenderIDInfo struct {
	AccountID string     `json:"accountId"`
	Limit     int        `json:"limit"`
	SenderIDs []SenderID `json:"senderIds"`
}

// DisplayStatus returns a human-friendly status for the sender ID.
func (s *SenderID) DisplayStatus() string {
	if s.Status == "" {
		return "-"
	}
	return s.Status
}

// DisplayMethod returns a human-friendly method for the sender ID.
func (s *SenderID) DisplayMethod() string {
	if s.Method == "" {
		return "-"
	}
	return s.Method
}

// DisplayExpireAt returns a formatted expiration date or "-".
func (s *SenderID) DisplayExpireAt() string {
	if s.ExpireAt == "" {
		return "-"
	}
	// Truncate to date only if it looks like ISO 8601
	if len(s.ExpireAt) >= 10 {
		return s.ExpireAt[:10]
	}
	return s.ExpireAt
}
