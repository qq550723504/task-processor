package listingkit

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	"task-processor/internal/catalog"
	"task-processor/internal/catalog/canonical"
	"task-processor/internal/productimage"
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

type ListingKitResult struct {
	TaskID                      string                           `json:"task_id"`
	Status                      string                           `json:"status"`
	ReviewReasons               []string                         `json:"review_reasons,omitempty"`
	Platforms                   []string                         `json:"platforms,omitempty"`
	Country                     string                           `json:"country,omitempty"`
	Language                    string                           `json:"language,omitempty"`
	CatalogProduct              *catalog.Product                 `json:"catalog_product,omitempty"`
	AssetBundle                 *asset.Bundle                    `json:"asset_bundle,omitempty"`
	AssetInventorySummary       *asset.InventorySummary          `json:"asset_inventory_summary,omitempty"`
	AssetRenderPreviews         []AssetRenderPreview             `json:"asset_render_previews,omitempty"`
	PlatformAssetRenderPreviews []PlatformAssetRenderPreviews    `json:"platform_asset_render_previews,omitempty"`
	AssetGenerationSummary      *AssetGenerationSummary          `json:"asset_generation_summary,omitempty"`
	AssetGenerationTasks        []assetgeneration.Task           `json:"asset_generation_tasks,omitempty"`
	AssetGenerationQueue        *GenerationWorkQueue             `json:"asset_generation_queue,omitempty"`
	AssetGenerationOverview     *AssetGenerationOverview         `json:"asset_generation_overview,omitempty"`
	ReviewSummary               *GenerationReviewSummary         `json:"review_summary,omitempty"`
	ReviewRecords               []GenerationReviewRecord         `json:"review_records,omitempty"`
	CanonicalProduct            *canonical.Product               `json:"canonical_product,omitempty"`
	ImageAssets                 *productimage.ImageProcessResult `json:"image_assets,omitempty"`
	SDSSync                     *SDSSyncSummary                  `json:"sds_sync,omitempty"`
	Amazon                      *AmazonPackage                   `json:"amazon,omitempty"`
	Shein                       *sheinpub.Package                `json:"shein,omitempty"`
	Temu                        *TemuPackage                     `json:"temu,omitempty"`
	Walmart                     *WalmartPackage                  `json:"walmart,omitempty"`
	Summary                     *GenerationSummary               `json:"summary,omitempty"`
	Revision                    *ListingKitRevisionSummary       `json:"revision,omitempty"`
	RevisionHistoryTotal        int                              `json:"revision_history_total,omitempty"`
	RevisionHistory             []ListingKitRevisionRecord       `json:"revision_history,omitempty"`
	ChildTasks                  []ChildTaskState                 `json:"child_tasks,omitempty"`
	WorkflowStages              []WorkflowStage                  `json:"workflow_stages,omitempty"`
	WorkflowIssues              []WorkflowIssue                  `json:"workflow_issues,omitempty"`
	CreatedAt                   time.Time                        `json:"created_at"`
	UpdatedAt                   time.Time                        `json:"updated_at"`
}

type GenerationSummary struct {
	SourceType    string   `json:"source_type,omitempty"`
	ImageCount    int      `json:"image_count"`
	VariantCount  int      `json:"variant_count"`
	NeedsReview   bool     `json:"needs_review"`
	Warnings      []string `json:"warnings,omitempty"`
	IssueCount    int      `json:"issue_count,omitempty"`
	WarningCount  int      `json:"warning_count,omitempty"`
	ReviewCount   int      `json:"review_count,omitempty"`
	BlockingCount int      `json:"blocking_count,omitempty"`
}

type SDSSyncSummary struct {
	VariantID        int64               `json:"variant_id"`
	ProductID        int64               `json:"product_id,omitempty"`
	PrototypeGroupID int64               `json:"prototype_group_id,omitempty"`
	LayerID          string              `json:"layer_id,omitempty"`
	MaterialID       int64               `json:"material_id,omitempty"`
	ProductName      string              `json:"product_name,omitempty"`
	ProductSKU       string              `json:"product_sku,omitempty"`
	VariantSKU       string              `json:"variant_sku,omitempty"`
	VariantSize      string              `json:"variant_size,omitempty"`
	VariantColor     string              `json:"variant_color,omitempty"`
	MockupImageURLs  []string            `json:"mockup_image_urls,omitempty"`
	VariantResults   []SDSSyncSummary    `json:"variant_results,omitempty"`
	Status           string              `json:"status,omitempty"`
	Error            string              `json:"error,omitempty"`
	Diagnostics      *SDSSyncDiagnostics `json:"diagnostics,omitempty"`
}

type SDSSyncDiagnostics struct {
	MaterialImageURL string                             `json:"material_image_url,omitempty"`
	MaterialFileCode string                             `json:"material_file_code,omitempty"`
	MaterialWidth    int                                `json:"material_width,omitempty"`
	MaterialHeight   int                                `json:"material_height,omitempty"`
	LayerContent     string                             `json:"layer_content,omitempty"`
	LayerImgWidth    int                                `json:"layer_img_width,omitempty"`
	LayerImgHeight   int                                `json:"layer_img_height,omitempty"`
	ResizeMode       int                                `json:"resize_mode"`
	FitLevel         float64                            `json:"fit_level,omitempty"`
	RenderedCount    int                                `json:"rendered_count"`
	FinishedProduct  *SDSSyncFinishedProductObservation `json:"finished_product,omitempty"`
	SensitiveWords   []SDSSyncSensitiveWordHit          `json:"sensitive_words,omitempty"`
}

type SDSSyncFinishedProductObservation struct {
	Found             bool   `json:"found,omitempty"`
	BuildFinish       bool   `json:"build_finish,omitempty"`
	Status            int    `json:"status,omitempty"`
	MaterialImageName string `json:"material_image_name,omitempty"`
	TaskID            string `json:"task_id,omitempty"`
	DesignTaskID      string `json:"design_task_id,omitempty"`
	ItemID            string `json:"item_id,omitempty"`
	ImageCount        int    `json:"image_count,omitempty"`
	ThumbnailCount    int    `json:"thumbnail_count,omitempty"`
}

type SDSSyncSensitiveWordHit struct {
	SensitiveWord string `json:"sensitive_word,omitempty"`
	Type          int    `json:"type,omitempty"`
	TypeStrs      string `json:"type_strs,omitempty"`
	ImgURL        string `json:"img_url,omitempty"`
	IsParent      int    `json:"is_parent,omitempty"`
	PositionStrs  string `json:"position_strs,omitempty"`
}

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

func (r GenerateRequest) Value() (driver.Value, error) { return json.Marshal(r) }

func (r *GenerateRequest) Scan(value any) error {
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, r)
}

func (r ListingKitResult) Value() (driver.Value, error) { return json.Marshal(r) }

func (r *ListingKitResult) Scan(value any) error {
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, r)
}
