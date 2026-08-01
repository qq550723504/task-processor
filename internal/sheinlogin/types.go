package sheinlogin

import "time"

type FailureSummary struct {
	ErrorCode            string    `json:"error_code,omitempty"`
	ErrorMessage         string    `json:"error_message,omitempty"`
	PageState            string    `json:"page_state,omitempty"`
	ActionKey            string    `json:"action_key,omitempty"`
	ActionMessage        string    `json:"action_message,omitempty"`
	ArtifactPath         string    `json:"artifact_path,omitempty"`
	CapturedAt           time.Time `json:"captured_at,omitempty"`
	Stage                string    `json:"stage,omitempty"`
	URL                  string    `json:"url,omitempty"`
	Title                string    `json:"title,omitempty"`
	LoginError           string    `json:"login_error,omitempty"`
	WaitingForVerifyCode bool      `json:"waiting_for_verify_code,omitempty"`
}

type FailureDetail struct {
	ErrorCode              string           `json:"error_code,omitempty"`
	ErrorMessage           string           `json:"error_message,omitempty"`
	PageState              string           `json:"page_state,omitempty"`
	ActionKey              string           `json:"action_key,omitempty"`
	ActionMessage          string           `json:"action_message,omitempty"`
	ArtifactPath           string           `json:"artifact_path,omitempty"`
	CapturedAt             time.Time        `json:"captured_at,omitempty"`
	Stage                  string           `json:"stage,omitempty"`
	URL                    string           `json:"url,omitempty"`
	Title                  string           `json:"title,omitempty"`
	LoginError             string           `json:"login_error,omitempty"`
	WaitingForVerifyCode   bool             `json:"waiting_for_verify_code,omitempty"`
	OnLoginPage            bool             `json:"on_login_page,omitempty"`
	RequestFailureModal    bool             `json:"request_failure_modal,omitempty"`
	LoginFormVisible       bool             `json:"login_form_visible,omitempty"`
	SellerHubVisible       bool             `json:"seller_hub_visible,omitempty"`
	VerificationVisible    bool             `json:"verification_visible,omitempty"`
	PermissionVisible      bool             `json:"permission_visible,omitempty"`
	AgreementVisible       bool             `json:"agreement_visible,omitempty"`
	CredentialErrorVisible bool             `json:"credential_error_visible,omitempty"`
	BodyText               string           `json:"body_text,omitempty"`
	SelectorStates         map[string]bool  `json:"selector_states,omitempty"`
	NetworkPayloads        []map[string]any `json:"network_payloads,omitempty"`
}

type RecommendedAction struct {
	Key     string `json:"key,omitempty"`
	Message string `json:"message,omitempty"`
}

type Account struct {
	StoreID   int64  `json:"store_id"`
	TenantID  int64  `json:"tenant_id"`
	Username  string `json:"username"`
	Password  string `json:"-"`
	LoginURL  string `json:"login_url"`
	Proxy     string `json:"proxy,omitempty"`
	ShopName  string `json:"shop_name,omitempty"`
	Platform  string `json:"platform"`
	StoreName string `json:"store_name,omitempty"`
}

type LoginRequest struct {
	ForceLogin bool  `json:"force_login"`
	Headless   *bool `json:"headless,omitempty"`
}

type LoginAttemptStatus string

const (
	LoginAttemptQueued            LoginAttemptStatus = "queued"
	LoginAttemptLaunching         LoginAttemptStatus = "launching"
	LoginAttemptWaitingVerifyCode LoginAttemptStatus = "waiting_verify_code"
	LoginAttemptSucceeded         LoginAttemptStatus = "succeeded"
	LoginAttemptFailed            LoginAttemptStatus = "failed"
	LoginAttemptCancelled         LoginAttemptStatus = "cancelled"
	LoginAttemptInterrupted       LoginAttemptStatus = "interrupted"
)

func (s LoginAttemptStatus) IsActive() bool {
	return s == LoginAttemptQueued || s == LoginAttemptLaunching || s == LoginAttemptWaitingVerifyCode
}

// LoginAttempt is the durable, credential-free contract between the API and
// the browser worker. The worker reloads the store account itself.
type LoginAttempt struct {
	ID          string             `json:"id"`
	TenantID    int64              `json:"tenant_id"`
	StoreID     int64              `json:"store_id"`
	ForceLogin  bool               `json:"force_login"`
	Headless    *bool              `json:"headless,omitempty"`
	Status      LoginAttemptStatus `json:"status"`
	WorkerID    string             `json:"worker_id,omitempty"`
	Message     string             `json:"message,omitempty"`
	ErrorCode   string             `json:"error_code,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
	StartedAt   *time.Time         `json:"started_at,omitempty"`
	CompletedAt *time.Time         `json:"completed_at,omitempty"`
}

type VerifyCodeRequest struct {
	Code          string `json:"code"`
	ExpireSeconds int    `json:"expire_seconds,omitempty"`
	AttemptID     string `json:"attempt_id,omitempty"`
}

type LoginResult struct {
	Success              bool               `json:"success"`
	Message              string             `json:"message"`
	StoreID              int64              `json:"store_id"`
	TenantID             int64              `json:"tenant_id"`
	Username             string             `json:"username,omitempty"`
	CookieTTL            int64              `json:"cookie_ttl,omitempty"`
	CookieCount          int                `json:"cookie_count,omitempty"`
	WaitingForVerifyCode bool               `json:"waiting_for_verify_code,omitempty"`
	ErrorCode            string             `json:"error_code,omitempty"`
	LoginType            string             `json:"login_type,omitempty"`
	LoginTime            time.Time          `json:"login_time,omitempty"`
	FailureArtifactPath  string             `json:"failure_artifact_path,omitempty"`
	LastFailure          *FailureSummary    `json:"last_failure,omitempty"`
	AttemptID            string             `json:"attempt_id,omitempty"`
	ExecutionStatus      LoginAttemptStatus `json:"execution_status,omitempty"`
}

type AccountStatus struct {
	Account              Account           `json:"account"`
	HasCookie            bool              `json:"has_cookie"`
	CookieTTL            int64             `json:"cookie_ttl,omitempty"`
	WaitingForVerifyCode bool              `json:"waiting_for_verify_code"`
	LastLoginTime        *time.Time        `json:"last_login_time,omitempty"`
	LoginInProgress      bool              `json:"login_in_progress"`
	LastFailure          *FailureSummary   `json:"last_failure,omitempty"`
	RecommendedAction    RecommendedAction `json:"recommended_action,omitempty"`
	LatestAttempt        *LoginAttempt     `json:"latest_attempt,omitempty"`
}

type WarehouseOption struct {
	WarehouseCode   string   `json:"warehouse_code,omitempty"`
	WarehouseName   string   `json:"warehouse_name,omitempty"`
	SaleCountryList []string `json:"sale_country_list,omitempty"`
	WarehouseType   int      `json:"warehouse_type,omitempty"`
}

type ServiceHealth struct {
	Initialized         bool `json:"initialized"`
	RedisReady          bool `json:"redis_ready"`
	ManagementReady     bool `json:"management_ready"`
	MaxConcurrentLogins int  `json:"max_concurrent_logins"`
}
