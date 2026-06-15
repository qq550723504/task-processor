package listingkit

import (
	"task-processor/internal/productimage"
)

type GenerateRequest struct {
	TenantID           string           `json:"tenant_id,omitempty"`
	UserID             string           `json:"user_id,omitempty"`
	ImageURLs          []string         `json:"image_urls,omitempty"`
	Text               string           `json:"text,omitempty"`
	ProductURL         string           `json:"product_url,omitempty"`
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
	ImageStrategy string                               `json:"image_strategy,omitempty"`
	ProcessImages bool                                 `json:"process_images"`
	Scene         *productimage.SceneGenerationOptions `json:"scene,omitempty"`
	SheinStudio   *SheinStudioOptions                  `json:"shein_studio,omitempty"`
	SDS           *SDSSyncOptions                      `json:"sds,omitempty"`
}
