package types

// Quota request status values returned by /quota/v1/me/system.
const (
	QuotaStatusPending  = "PENDING"
	QuotaStatusApproved = "APPROVED"
	QuotaStatusRejected = "REJECTED"
)

// QuotaInfo represents the current sending quota for the authenticated account.
// Mirrors GET /quota/v1/me response.
type QuotaInfo struct {
	AccountID      string `json:"accountId"`
	Quota          int    `json:"quota"`
	Min            int    `json:"min"`
	Max            int    `json:"max"`
	AutoAdjustment bool   `json:"autoAdjustment"`
	OverseasQuota  int    `json:"overseasQuota"`
	DateCreated    string `json:"dateCreated"`
	DateUpdated    string `json:"dateUpdated"`
}

// IncreaseQuotaRequest represents a single quota increase request record.
// Mirrors the IncreaseQuota document returned by POST /quota/v1/me/system
// and entries in GET /quota/v1/me/system increaseQuotaList.
type IncreaseQuotaRequest struct {
	HandleKey       string `json:"handleKey"`
	AccountID       string `json:"accountId"`
	Status          string `json:"status"`
	RequestedQuota  int    `json:"requestedQuota"`
	ReasonRequested string `json:"reasonRequested"`
	ReasonRejected  string `json:"reasonRejected"`
	DateCreated     string `json:"dateCreated"`
	DateUpdated     string `json:"dateUpdated"`
}

// IncreaseQuotaListResponse represents the paginated response from
// GET /quota/v1/me/system.
type IncreaseQuotaListResponse struct {
	IncreaseQuotaList []IncreaseQuotaRequest `json:"increaseQuotaList"`
	NextKey           string                 `json:"nextKey"`
}
