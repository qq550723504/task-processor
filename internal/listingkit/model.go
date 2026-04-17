package listingkit

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/asset"
	"task-processor/internal/catalog"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

var ErrTaskNotFound = errors.New("task not found")
var ErrTaskNotPending = errors.New("task is not pending")

type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
)

type GenerateRequest struct {
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

type GenerateOptions struct {
	ProcessImages bool `json:"process_images"`
}

type Task struct {
	ID         string            `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Request    *GenerateRequest  `json:"request" gorm:"type:text"`
	Status     TaskStatus        `json:"status" gorm:"type:varchar(20);index"`
	Result     *ListingKitResult `json:"result,omitempty" gorm:"type:text"`
	Error      string            `json:"error,omitempty" gorm:"type:text"`
	CreatedAt  time.Time         `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time         `json:"updated_at" gorm:"autoUpdateTime"`
	RetryCount int               `json:"retry_count" gorm:"default:0"`
}

type TaskResult struct {
	TaskID      string            `json:"task_id"`
	Status      TaskStatus        `json:"status"`
	Result      *ListingKitResult `json:"result,omitempty"`
	Error       string            `json:"error,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
}

type ChildTaskState struct {
	Kind   string `json:"kind"`
	TaskID string `json:"task_id,omitempty"`
	Status string `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
}

type ListingKitResult struct {
	TaskID               string                           `json:"task_id"`
	Status               string                           `json:"status"`
	Platforms            []string                         `json:"platforms,omitempty"`
	Country              string                           `json:"country,omitempty"`
	Language             string                           `json:"language,omitempty"`
	CatalogProduct       *catalog.Product                 `json:"catalog_product,omitempty"`
	AssetBundle          *asset.Bundle                    `json:"asset_bundle,omitempty"`
	CanonicalProduct     *productenrich.CanonicalProduct  `json:"canonical_product,omitempty"`
	ImageAssets          *productimage.ImageProcessResult `json:"image_assets,omitempty"`
	Amazon               *AmazonPackage                   `json:"amazon,omitempty"`
	Shein                *sheinpub.Package                `json:"shein,omitempty"`
	Temu                 *TemuPackage                     `json:"temu,omitempty"`
	Walmart              *WalmartPackage                  `json:"walmart,omitempty"`
	Summary              *GenerationSummary               `json:"summary,omitempty"`
	Revision             *ListingKitRevisionSummary       `json:"revision,omitempty"`
	RevisionHistoryTotal int                              `json:"revision_history_total,omitempty"`
	RevisionHistory      []ListingKitRevisionRecord       `json:"revision_history,omitempty"`
	ChildTasks           []ChildTaskState                 `json:"child_tasks,omitempty"`
	CreatedAt            time.Time                        `json:"created_at"`
	UpdatedAt            time.Time                        `json:"updated_at"`
}

type GenerationSummary struct {
	SourceType   string   `json:"source_type,omitempty"`
	ImageCount   int      `json:"image_count"`
	VariantCount int      `json:"variant_count"`
	NeedsReview  bool     `json:"needs_review"`
	Warnings     []string `json:"warnings,omitempty"`
}

type AmazonPackage struct {
	Draft *amazonlisting.AmazonListingDraft `json:"draft,omitempty"`
}

type TemuPackage struct {
	GoodsName          string            `json:"goods_name,omitempty"`
	CategoryPath       []string          `json:"category_path,omitempty"`
	ShortDescription   string            `json:"short_description,omitempty"`
	BulletPoints       []string          `json:"bullet_points,omitempty"`
	Attributes         map[string]string `json:"attributes,omitempty"`
	SkcList            []TemuSKCPackage  `json:"skc_list,omitempty"`
	BatchSkuInfo       *TemuBatchSKUInfo `json:"batch_sku_info,omitempty"`
	Images             *PlatformImageSet `json:"images,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	CategoryDisclaimer []string          `json:"category_disclaimer,omitempty"`
	ReviewNotes        []string          `json:"review_notes,omitempty"`
}

type WalmartPackage struct {
	ProductName      string            `json:"product_name,omitempty"`
	Brand            string            `json:"brand,omitempty"`
	ProductType      string            `json:"product_type,omitempty"`
	ShortDescription string            `json:"short_description,omitempty"`
	LongDescription  string            `json:"long_description,omitempty"`
	KeyFeatures      []string          `json:"key_features,omitempty"`
	Attributes       map[string]string `json:"attributes,omitempty"`
	Variants         []PlatformVariant `json:"variants,omitempty"`
	Images           *PlatformImageSet `json:"images,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	ReviewNotes      []string          `json:"review_notes,omitempty"`
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
