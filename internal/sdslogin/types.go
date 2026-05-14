package sdslogin

import "time"

type LoginRequest struct {
	ForceLogin bool   `json:"force_login"`
	Headless   *bool  `json:"headless,omitempty"`
	TargetURL  string `json:"target_url,omitempty"`
}

type ManualLoginRequest struct {
	TenantID     string `json:"tenant_id"`
	Identifier   string `json:"identifier"`
	MerchantName string `json:"merchant_name"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	ForceLogin   bool   `json:"force_login"`
	Headless     *bool  `json:"headless,omitempty"`
	TargetURL    string `json:"target_url,omitempty"`
}

type AuthPayload struct {
	TenantID     string         `json:"tenant_id"`
	ShopID       string         `json:"shop_id"`
	Identifier   string         `json:"identifier"`
	Username     string         `json:"username"`
	MerchantName string         `json:"merchant_name"`
	AccessToken  string         `json:"access_token"`
	OutToken     string         `json:"out_token,omitempty"`
	MerchantID   int64          `json:"merchant_id,omitempty"`
	UserID       int64          `json:"user_id,omitempty"`
	Cookies      []CookieRecord `json:"cookies,omitempty"`
	BrowserState map[string]any `json:"browser_state,omitempty"`
	IssuedAt     time.Time      `json:"issued_at"`
	Source       string         `json:"source,omitempty"`
	CurrentURL   string         `json:"current_url,omitempty"`
}

type CookieRecord struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain,omitempty"`
	Path     string    `json:"path,omitempty"`
	Expires  time.Time `json:"expires,omitempty"`
	Secure   bool      `json:"secure,omitempty"`
	HTTPOnly bool      `json:"httpOnly,omitempty"`
}

type Status struct {
	TenantID             string     `json:"tenant_id"`
	Identifier           string     `json:"identifier"`
	MerchantName         string     `json:"merchant_name,omitempty"`
	Username             string     `json:"username,omitempty"`
	HasCookie            bool       `json:"has_cookie"`
	HasAccessToken       bool       `json:"has_access_token"`
	MerchantID           int64      `json:"merchant_id,omitempty"`
	IssuedAt             *time.Time `json:"issued_at,omitempty"`
	Source               string     `json:"source,omitempty"`
	WaitingForVerifyCode bool       `json:"waiting_for_verify_code"`
	LoginInProgress      bool       `json:"login_in_progress"`
	LastError            string     `json:"last_error,omitempty"`
}

type ServiceHealth struct {
	Initialized        bool `json:"initialized"`
	MaxConcurrentLogin int  `json:"max_concurrent_logins"`
	AdminPageEnabled   bool `json:"admin_page_enabled"`
}
