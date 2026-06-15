package listingkit

import (
	"task-processor/internal/amazonlisting"
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

type GenerationRecoverySummary struct {
	Title                  string                              `json:"title,omitempty"`
	Summary                string                              `json:"summary,omitempty"`
	Severity               string                              `json:"severity,omitempty"`
	Urgency                string                              `json:"urgency,omitempty"`
	CTAKind                string                              `json:"cta_kind,omitempty"`
	ActionKey              string                              `json:"action_key,omitempty"`
	RecommendedCount       int                                 `json:"recommended_count"`
	PrimaryDescriptor      *GenerationPanelResourceDescriptor  `json:"primary_descriptor,omitempty"`
	RecommendedDescriptors []GenerationPanelResourceDescriptor `json:"recommended_descriptors,omitempty"`
}

type GenerationResolvedActionSummary struct {
	SourceKind       string                            `json:"source_kind,omitempty"`
	Title            string                            `json:"title,omitempty"`
	Summary          string                            `json:"summary,omitempty"`
	CTAKind          string                            `json:"cta_kind,omitempty"`
	ActionKey        string                            `json:"action_key,omitempty"`
	NavigationTarget *GenerationReviewNavigationTarget `json:"navigation_target,omitempty"`
	ActionTarget     *AssetGenerationActionTarget      `json:"action_target,omitempty"`
	RecoverySummary  *GenerationRecoverySummary        `json:"recovery_summary,omitempty"`
}

type AmazonPackage struct {
	Draft       *amazonlisting.AmazonListingDraft `json:"draft,omitempty"`
	ImageBundle *common.PublishImageBundle        `json:"image_bundle,omitempty"`
}

type TemuPackage struct {
	GoodsName          string                     `json:"goods_name,omitempty"`
	CategoryPath       []string                   `json:"category_path,omitempty"`
	ShortDescription   string                     `json:"short_description,omitempty"`
	BulletPoints       []string                   `json:"bullet_points,omitempty"`
	Attributes         map[string]string          `json:"attributes,omitempty"`
	SkcList            []TemuSKCPackage           `json:"skc_list,omitempty"`
	BatchSkuInfo       *TemuBatchSKUInfo          `json:"batch_sku_info,omitempty"`
	Images             *PlatformImageSet          `json:"images,omitempty"`
	ImageBundle        *common.PublishImageBundle `json:"image_bundle,omitempty"`
	Metadata           map[string]string          `json:"metadata,omitempty"`
	CategoryDisclaimer []string                   `json:"category_disclaimer,omitempty"`
	ReviewNotes        []string                   `json:"review_notes,omitempty"`
}

type WalmartPackage struct {
	ProductName      string                     `json:"product_name,omitempty"`
	Brand            string                     `json:"brand,omitempty"`
	ProductType      string                     `json:"product_type,omitempty"`
	ShortDescription string                     `json:"short_description,omitempty"`
	LongDescription  string                     `json:"long_description,omitempty"`
	KeyFeatures      []string                   `json:"key_features,omitempty"`
	Attributes       map[string]string          `json:"attributes,omitempty"`
	Variants         []PlatformVariant          `json:"variants,omitempty"`
	Images           *PlatformImageSet          `json:"images,omitempty"`
	ImageBundle      *common.PublishImageBundle `json:"image_bundle,omitempty"`
	Metadata         map[string]string          `json:"metadata,omitempty"`
	ReviewNotes      []string                   `json:"review_notes,omitempty"`
}

type PlatformVariant = common.Variant
type PlatformPrice = common.Price
type PlatformImageSet = common.ImageSet
type PlatformAttribute = common.Attribute
type PlatformSite = common.Site

type SheinSKCPackage = sheinpub.SKCPackage

type TemuSKCPackage struct {
	Priority        int               `json:"priority,omitempty"`
	ColorImageURL   string            `json:"color_image_url,omitempty"`
	Spec            []TemuSpecPackage `json:"spec,omitempty"`
	CarouselGallery []string          `json:"carousel_gallery,omitempty"`
	SKUs            []PlatformVariant `json:"skus,omitempty"`
}

type TemuSpecPackage struct {
	Name       string `json:"name,omitempty"`
	Value      string `json:"value,omitempty"`
	ParentName string `json:"parent_name,omitempty"`
}

type TemuBatchSKUInfo struct {
	Currency  string `json:"currency,omitempty"`
	Quantity  string `json:"quantity,omitempty"`
	OutSkuSN  string `json:"out_sku_sn,omitempty"`
	Weight    string `json:"weight,omitempty"`
	Length    string `json:"length,omitempty"`
	Width     string `json:"width,omitempty"`
	Height    string `json:"height,omitempty"`
	Price     string `json:"price,omitempty"`
	CostPrice string `json:"cost_price,omitempty"`
}
