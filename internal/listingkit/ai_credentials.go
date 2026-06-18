package listingkit

import "time"

type AIClientCredential struct {
	TenantID      string
	UserID        string
	ClientName    string
	APIKey        string
	BaseURL       string
	Model         string
	TimeoutSecond int
	Enabled       bool
	UpdatedAt     time.Time
}
