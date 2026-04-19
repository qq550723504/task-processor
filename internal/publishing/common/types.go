package common

import (
	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
)

type Variant struct {
	SKU        string            `json:"sku,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Price      *Price            `json:"price,omitempty"`
	Stock      int               `json:"stock,omitempty"`
	Image      string            `json:"image,omitempty"`
	Barcode    string            `json:"barcode,omitempty"`
	IsDefault  bool              `json:"is_default,omitempty"`
}

type Price struct {
	Currency  string  `json:"currency,omitempty"`
	Amount    float64 `json:"amount,omitempty"`
	CostPrice float64 `json:"cost_price,omitempty"`
}

type ImageSet struct {
	MainImage    string   `json:"main_image,omitempty"`
	WhiteBgImage string   `json:"white_bg_image,omitempty"`
	Gallery      []string `json:"gallery,omitempty"`
	SourceImages []string `json:"source_images,omitempty"`
}

type PublishImageBundle struct {
	Platform          string                 `json:"platform,omitempty"`
	Main              *BundleSlot            `json:"main,omitempty"`
	Gallery           []BundleSlot           `json:"gallery,omitempty"`
	Auxiliary         []BundleSlot           `json:"auxiliary,omitempty"`
	MissingSlots      []MissingSlot          `json:"missing_slots,omitempty"`
	PendingGeneration []assetgeneration.Task `json:"pending_generation,omitempty"`
	Warnings          []string               `json:"warnings,omitempty"`
	RecipeIDs         []string               `json:"recipe_ids,omitempty"`
	SelectedAssetIDs  []string               `json:"selected_asset_ids,omitempty"`
}

type BundleSlot struct {
	Key             string   `json:"key,omitempty"`
	Purpose         string   `json:"purpose,omitempty"`
	IdealKind       string   `json:"ideal_kind,omitempty"`
	TemplateLabel   string   `json:"template_label,omitempty"`
	StateLabel      string   `json:"state_label,omitempty"`
	RetryHint       string   `json:"retry_hint,omitempty"`
	AssetID         string   `json:"asset_id,omitempty"`
	URL             string   `json:"url,omitempty"`
	Kind            string   `json:"kind,omitempty"`
	RecipeID        string   `json:"recipe_id,omitempty"`
	SatisfiedBy     string   `json:"satisfied_by,omitempty"`
	FallbackFrom    string   `json:"fallback_from,omitempty"`
	ExecutionStatus string   `json:"execution_status,omitempty"`
	SourceAssetIDs  []string `json:"source_asset_ids,omitempty"`
}

type BundleSelectionRule struct {
	Platform       string       `json:"platform,omitempty"`
	Slot           string       `json:"slot,omitempty"`
	Optional       bool         `json:"optional,omitempty"`
	MaxItems       int          `json:"max_items,omitempty"`
	PreferredKinds []asset.Kind `json:"preferred_kinds,omitempty"`
}

type MissingSlot struct {
	Slot          string `json:"slot,omitempty"`
	Purpose       string `json:"purpose,omitempty"`
	RecipeID      string `json:"recipe_id,omitempty"`
	TemplateLabel string `json:"template_label,omitempty"`
	RenderProfile string `json:"render_profile,omitempty"`
	StateLabel    string `json:"state_label,omitempty"`
	Reason        string `json:"reason,omitempty"`
	Optional      bool   `json:"optional,omitempty"`
}

type Attribute struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type Site struct {
	MainSite string   `json:"main_site,omitempty"`
	SubSites []string `json:"sub_sites,omitempty"`
}
