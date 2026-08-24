package listingkit

import (
	"task-processor/internal/productimage"
)

type SourceReference struct {
	Key      string `json:"key,omitempty"`
	Type     string `json:"type,omitempty"`
	Platform string `json:"platform,omitempty"`
	ID       string `json:"id,omitempty"`
	URL      string `json:"url,omitempty"`
}

type GenerateRequest struct {
	TenantID string `json:"tenant_id,omitempty"`
	// BillingTenantID is set only by the authenticated HTTP boundary after its
	// subscription check. It is intentionally excluded from the persisted
	// request JSON; Task owns the durable billing identity separately from the
	// caller-facing ownership tenant.
	BillingTenantID    string           `json:"-"`
	UserID             string           `json:"user_id,omitempty"`
	ImageURLs          []string         `json:"image_urls,omitempty"`
	Text               string           `json:"text,omitempty"`
	ProductURL         string           `json:"product_url,omitempty"`
	Source             *SourceReference `json:"source,omitempty"`
	Platforms          []string         `json:"platforms,omitempty"`
	Country            string           `json:"country,omitempty"`
	Language           string           `json:"language,omitempty"`
	SheinStoreID       int64            `json:"shein_store_id,omitempty"`
	TargetCategoryHint string           `json:"target_category_hint,omitempty"`
	BrandHint          string           `json:"brand_hint,omitempty"`
	Options            *GenerateOptions `json:"options,omitempty"`
}

type WarmSDSBaselineRequest struct {
	TenantID  string          `json:"tenant_id,omitempty"`
	ImageURLs []string        `json:"image_urls,omitempty"`
	SDS       *SDSSyncOptions `json:"sds,omitempty"`
}

type GenerateOptions struct {
	ImageStrategy               string                               `json:"image_strategy,omitempty"`
	ProcessImages               bool                                 `json:"process_images"`
	CompatibilityTargetPlatform string                               `json:"compatibility_target_platform,omitempty"`
	Scene                       *productimage.SceneGenerationOptions `json:"scene,omitempty"`
	SheinStudio                 *SheinStudioOptions                  `json:"shein_studio,omitempty"`
	SDS                         *SDSSyncOptions                      `json:"sds,omitempty"`
}
