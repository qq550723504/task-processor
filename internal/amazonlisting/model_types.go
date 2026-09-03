package amazonlisting

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	amazonmodel "task-processor/internal/marketplace/amazon/model"
	"task-processor/internal/product/catalog"
	"task-processor/internal/shared/aiidentity"
)

var ErrTaskNotFound = errors.New("task not found")
var ErrTaskNotPending = errors.New("task is not pending")
var ErrProductSnapshotNotReady = errors.New("product snapshot is not ready")

type TaskStatus = amazonmodel.TaskStatus

const (
	TaskStatusPending     = amazonmodel.TaskStatusPending
	TaskStatusProcessing  = amazonmodel.TaskStatusProcessing
	TaskStatusCompleted   = amazonmodel.TaskStatusCompleted
	TaskStatusNeedsReview = amazonmodel.TaskStatusNeedsReview
	TaskStatusRejected    = amazonmodel.TaskStatusRejected
	TaskStatusFailed      = amazonmodel.TaskStatusFailed
)

type GenerateRequest struct {
	Marketplace        string           `json:"marketplace"`
	Country            string           `json:"country,omitempty"`
	Language           string           `json:"language,omitempty"`
	ProductKey         string           `json:"product_key"`
	TargetCategoryHint string           `json:"target_category_hint,omitempty"`
	BrandHint          string           `json:"brand_hint,omitempty"`
	Options            *GenerateOptions `json:"options,omitempty"`
}

type GenerateOptions struct {
	StrictValidation bool `json:"strict_validation"`
}

type ReviewTaskRequest struct {
	Action string           `json:"action"`
	Reason string           `json:"reason,omitempty"`
	Edits  []DraftFieldEdit `json:"edits,omitempty"`
}

type DraftFieldEdit struct {
	Field       string   `json:"field"`
	StringValue string   `json:"string_value,omitempty"`
	StringList  []string `json:"string_list,omitempty"`
	NumberValue *float64 `json:"number_value,omitempty"`
}

type SubmitTaskRequest struct {
	Action string `json:"action"`
}

type Task struct {
	ID                                    string `json:"id" gorm:"primaryKey;type:varchar(36)"`
	aiidentity.PersistedExecutionEnvelope `gorm:"embedded"`
	Request                               *GenerateRequest    `json:"request" gorm:"type:text"`
	Status                                TaskStatus          `json:"status" gorm:"type:varchar(20);index"`
	Result                                *AmazonListingDraft `json:"result,omitempty" gorm:"type:text"`
	Error                                 string              `json:"error,omitempty" gorm:"type:text"`
	CreatedAt                             time.Time           `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                             time.Time           `json:"updated_at" gorm:"autoUpdateTime"`
	RetryCount                            int                 `json:"retry_count" gorm:"default:0"`
	SourceSnapshotVersion                 uint64              `json:"source_snapshot_version,omitempty" gorm:"index"`
}

func (t *Task) ExecutionEnvelope() (aiidentity.ExecutionEnvelope, error) {
	if t == nil {
		return aiidentity.ExecutionEnvelope{}, aiidentity.ErrIdentityIntegrity
	}
	return t.PersistedExecutionEnvelope.ExecutionEnvelope(t.ID)
}

func (t *Task) SetExecutionEnvelope(envelope aiidentity.ExecutionEnvelope) {
	if t == nil {
		return
	}
	t.PersistedExecutionEnvelope = aiidentity.PersistedExecutionEnvelopeFrom(envelope)
}

type TaskResult struct {
	TaskID      string              `json:"task_id"`
	Status      TaskStatus          `json:"status"`
	Result      *AmazonListingDraft `json:"result,omitempty"`
	Error       string              `json:"error,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
	CompletedAt *time.Time          `json:"completed_at,omitempty"`
}

type TaskWorkbench = amazonmodel.TaskWorkbench
type TaskQueueQuery = amazonmodel.TaskQueueQuery
type TaskQueueResult = amazonmodel.TaskQueueResult
type ReviewItemSummary = amazonmodel.ReviewItemSummary
type WorkbenchActionBox = amazonmodel.WorkbenchActionBox

type AmazonListingDraft struct {
	TaskID             string                  `json:"task_id"`
	Status             string                  `json:"status"`
	Marketplace        string                  `json:"marketplace"`
	Country            string                  `json:"country,omitempty"`
	Language           string                  `json:"language,omitempty"`
	Source             AmazonSourceTrace       `json:"source"`
	ProductType        string                  `json:"product_type,omitempty"`
	CategoryPath       []string                `json:"category_path,omitempty"`
	BrowseNode         string                  `json:"browse_node,omitempty"`
	Brand              string                  `json:"brand,omitempty"`
	Title              string                  `json:"title,omitempty"`
	BulletPoints       []string                `json:"bullet_points,omitempty"`
	Description        string                  `json:"description,omitempty"`
	SearchTerms        []string                `json:"search_terms,omitempty"`
	Attributes         map[string]string       `json:"attributes,omitempty"`
	RequiredAttributes map[string]string       `json:"required_attributes,omitempty"`
	Dimensions         *AmazonDimensions       `json:"dimensions,omitempty"`
	Weight             *AmazonWeight           `json:"weight,omitempty"`
	Package            *AmazonPackageInfo      `json:"package,omitempty"`
	Variants           []AmazonVariantDraft    `json:"variants,omitempty"`
	Images             *AmazonImageBundle      `json:"images,omitempty"`
	Pricing            *AmazonPricingDraft     `json:"pricing,omitempty"`
	Compliance         *AmazonComplianceReport `json:"compliance,omitempty"`
	ListingIPRisk      *IPRiskReport           `json:"listing_ip_risk,omitempty"`
	Review             *AmazonReviewReport     `json:"review,omitempty"`
	Export             *AmazonListingExport    `json:"export,omitempty"`
	Submission         *AmazonSubmissionReport `json:"submission,omitempty"`
	LastAmazonIssues   []AmazonIssue           `json:"last_amazon_issues,omitempty"`
	FixHistory         []AmazonFixRecord       `json:"fix_history,omitempty"`
	ReviewItems        []AmazonReviewItem      `json:"review_items,omitempty"`
	CreatedAt          time.Time               `json:"created_at"`
	UpdatedAt          time.Time               `json:"updated_at"`
}

type AmazonSourceTrace struct {
	ProductKey       string                 `json:"product_key"`
	SnapshotSources  []catalog.SourceRecord `json:"snapshot_sources,omitempty"`
	ApprovedAssetIDs []string               `json:"approved_asset_ids,omitempty"`
}

type AmazonDimensions struct {
	Length float64 `json:"length"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Unit   string  `json:"unit"`
}

type AmazonWeight struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

type AmazonPackageInfo struct {
	Dimensions *AmazonDimensions `json:"dimensions,omitempty"`
	Weight     *AmazonWeight     `json:"weight,omitempty"`
	Quantity   int               `json:"quantity,omitempty"`
}

type AmazonMoney struct {
	Currency string  `json:"currency"`
	Amount   float64 `json:"amount"`
}

type AmazonVariantDraft struct {
	SKU        string            `json:"sku"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Price      *AmazonMoney      `json:"price,omitempty"`
	CostPrice  *AmazonMoney      `json:"cost_price,omitempty"`
	Inventory  int               `json:"inventory,omitempty"`
	Barcode    string            `json:"barcode,omitempty"`
	MainImage  string            `json:"main_image,omitempty"`
	IsDefault  bool              `json:"is_default,omitempty"`
}

type AmazonImageBundle struct {
	MainImage     string   `json:"main_image,omitempty"`
	WhiteBgImage  string   `json:"white_bg_image,omitempty"`
	GalleryImages []string `json:"gallery_images,omitempty"`
}

type AmazonPricingDraft struct {
	Currency       string  `json:"currency"`
	SourceCost     float64 `json:"source_cost,omitempty"`
	SuggestedPrice float64 `json:"suggested_price,omitempty"`
	MinPrice       float64 `json:"min_price,omitempty"`
	MarginRate     float64 `json:"margin_rate,omitempty"`
}

type AmazonComplianceReport struct {
	Ready          bool     `json:"ready"`
	BlockingIssues []string `json:"blocking_issues,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}

type IPRiskReport struct {
	Level   string   `json:"level"`
	Score   float64  `json:"score"`
	Reasons []string `json:"reasons,omitempty"`
}

type AmazonReviewReport struct {
	NeedsReview bool     `json:"needs_review"`
	Reasons     []string `json:"reasons,omitempty"`
}

type AmazonReviewItem = amazonmodel.AmazonReviewItem
type AmazonReviewEvidence = amazonmodel.AmazonReviewEvidence

type AmazonIssue = amazonmodel.AmazonIssue
type AmazonFixRecord = amazonmodel.AmazonFixRecord
type AmazonFixEvaluation = amazonmodel.AmazonFixEvaluation
type AmazonIssueSummary = amazonmodel.AmazonIssueSummary
type AmazonListingExport = amazonmodel.AmazonListingExport
type AmazonSubmissionReport = amazonmodel.AmazonSubmissionReport
type AmazonSubmissionRecord = amazonmodel.AmazonSubmissionRecord
type AmazonListingsAPIExport = amazonmodel.AmazonListingsAPIExport
type AmazonListingsPatchPayload = amazonmodel.AmazonListingsPatchPayload
type AmazonListingsPatchAction = amazonmodel.AmazonListingsPatchAction

const (
	OperatorActionFillBrand       = amazonmodel.OperatorActionFillBrand
	OperatorActionEditBrand       = amazonmodel.OperatorActionEditBrand
	OperatorActionFillBullets     = amazonmodel.OperatorActionFillBullets
	OperatorActionEditBullets     = amazonmodel.OperatorActionEditBullets
	OperatorActionEditTitle       = amazonmodel.OperatorActionEditTitle
	OperatorActionFillMainImage   = amazonmodel.OperatorActionFillMainImage
	OperatorActionFillImages      = amazonmodel.OperatorActionFillImages
	OperatorActionFillPrice       = amazonmodel.OperatorActionFillPrice
	OperatorActionEditPrice       = amazonmodel.OperatorActionEditPrice
	OperatorActionFillSKU         = amazonmodel.OperatorActionFillSKU
	OperatorActionEditSKU         = amazonmodel.OperatorActionEditSKU
	OperatorActionCheckCompliance = amazonmodel.OperatorActionCheckCompliance
	OperatorActionCheckHazmat     = amazonmodel.OperatorActionCheckHazmat
	OperatorActionEditCategory    = amazonmodel.OperatorActionEditCategory
	OperatorActionFillAttributes  = amazonmodel.OperatorActionFillAttributes
	OperatorActionManualReview    = amazonmodel.OperatorActionManualReview
)

func (r GenerateRequest) Value() (driver.Value, error) {
	return json.Marshal(r)
}

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

func (r AmazonListingDraft) Value() (driver.Value, error) {
	return json.Marshal(r)
}

func (r *AmazonListingDraft) Scan(value any) error {
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
