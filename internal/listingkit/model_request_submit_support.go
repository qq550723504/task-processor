package listingkit

import (
	"time"

	sheinpub "task-processor/internal/publishing/shein"
)

type SubmitTaskRequest struct {
	Platform       string `json:"platform,omitempty"`
	Action         string `json:"action,omitempty"`
	ConfirmedFinal bool   `json:"confirmed_final,omitempty"`
	RequestID      string `json:"request_id,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type SheinSettings struct {
	DefaultStoreID    int64                `json:"default_store_id,omitempty"`
	AvailableStores   []SheinStoreOption   `json:"available_stores,omitempty"`
	Site              string               `json:"site,omitempty"`
	WarehouseCode     string               `json:"warehouse_code,omitempty"`
	DefaultStock      int                  `json:"default_stock,omitempty"`
	DefaultSubmitMode string               `json:"default_submit_mode,omitempty"`
	Pricing           sheinpub.PricingRule `json:"pricing,omitempty"`
	UpdatedAt         *time.Time           `json:"updated_at,omitempty"`
}

type SheinStoreOption struct {
	ID       int64  `json:"id"`
	StoreID  string `json:"store_id,omitempty"`
	Name     string `json:"name,omitempty"`
	Platform string `json:"platform,omitempty"`
	Region   string `json:"region,omitempty"`
}

type AIClientSettings struct {
	Scope         string `json:"scope,omitempty"`
	ClientName    string `json:"client_name,omitempty"`
	APIKey        string `json:"api_key,omitempty"`
	APIKeySet     bool   `json:"api_key_set"`
	BaseURL       string `json:"base_url,omitempty"`
	Model         string `json:"model,omitempty"`
	APIStyle      string `json:"api_style,omitempty"`
	TimeoutSecond int    `json:"timeout_second,omitempty"`
	Enabled       bool   `json:"enabled"`
	UpdatedAt     string `json:"updated_at,omitempty"`
	ResolvedScope string `json:"resolved_scope,omitempty"`
}

type SheinPricePreviewRequest struct {
	Rule            *sheinpub.PricingRule `json:"rule,omitempty"`
	ManualOverrides map[string]float64    `json:"manual_price_overrides,omitempty"`
	ApplyToTask     bool                  `json:"apply_to_task,omitempty"`
}

type SheinFinalDraftUpdateRequest struct {
	Confirmed            *bool              `json:"confirmed,omitempty"`
	SubmitMode           string             `json:"submit_mode,omitempty"`
	ManualPriceOverrides map[string]float64 `json:"manual_price_overrides,omitempty"`
	FinalImageOrder      *[]string          `json:"final_image_order,omitempty"`
	MainImageURL         string             `json:"main_image_url,omitempty"`
	DeletedImageURLs     *[]string          `json:"deleted_image_urls,omitempty"`
	ImageRoleOverrides   map[string]string  `json:"image_role_overrides,omitempty"`
}

type SheinSubmissionEventPage struct {
	TaskID string                     `json:"task_id"`
	Items  []sheinpub.SubmissionEvent `json:"items,omitempty"`
}

type SheinResolutionCacheClearResult struct {
	TaskID       string   `json:"task_id"`
	Kind         string   `json:"kind"`
	DeletedKinds []string `json:"deleted_kinds,omitempty"`
}

type SheinCategorySearchCandidate struct {
	CategoryID     int      `json:"category_id"`
	CategoryIDList []int    `json:"category_id_list,omitempty"`
	CategoryPath   []string `json:"category_path,omitempty"`
	ProductTypeID  int      `json:"product_type_id,omitempty"`
	TopCategoryID  int      `json:"top_category_id,omitempty"`
	Source         string   `json:"source,omitempty"`
	MatchReason    string   `json:"match_reason,omitempty"`
}

type SheinCategorySearchResult struct {
	TaskID string                         `json:"task_id"`
	Query  string                         `json:"query"`
	Items  []SheinCategorySearchCandidate `json:"items,omitempty"`
}
