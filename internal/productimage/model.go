package productimage

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	productenrich "task-processor/internal/productenrich"
)

var ErrTaskNotFound = errors.New("task not found")

type TaskStatus string

const (
	TaskStatusPending     TaskStatus = "pending"
	TaskStatusProcessing  TaskStatus = "processing"
	TaskStatusCompleted   TaskStatus = "completed"
	TaskStatusNeedsReview TaskStatus = "needs_review"
	TaskStatusRejected    TaskStatus = "rejected"
	TaskStatusFailed      TaskStatus = "failed"
)

type AssetType string

const (
	AssetTypeMainImage     AssetType = "main_image"
	AssetTypeWhiteBgImage  AssetType = "white_bg_image"
	AssetTypeSubjectCutout AssetType = "subject_cutout"
	AssetTypeGalleryImage  AssetType = "gallery_image"
	AssetTypeSourceImage   AssetType = "source_image"
)

type ImageProcessRequest struct {
	ProductURL  string   `json:"product_url,omitempty"`
	ImageURLs   []string `json:"image_urls,omitempty"`
	Text        string   `json:"text,omitempty"`
	Marketplace string   `json:"marketplace"`
	Country     string   `json:"country,omitempty"`
}

type ReviewTaskRequest struct {
	Action string `json:"action"`
	Reason string `json:"reason,omitempty"`
}

type Task struct {
	ID         string               `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Request    *ImageProcessRequest `json:"request" gorm:"type:text"`
	Status     TaskStatus           `json:"status" gorm:"type:varchar(20);index"`
	Result     *ImageProcessResult  `json:"result,omitempty" gorm:"type:text"`
	Error      string               `json:"error,omitempty" gorm:"type:text"`
	CreatedAt  time.Time            `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time            `json:"updated_at" gorm:"autoUpdateTime"`
	RetryCount int                  `json:"retry_count" gorm:"default:0"`
}

type SourceBundle struct {
	Images      []string                       `json:"images"`
	Text        string                         `json:"text,omitempty"`
	ProductURL  string                         `json:"product_url,omitempty"`
	Marketplace string                         `json:"marketplace,omitempty"`
	Country     string                         `json:"country,omitempty"`
	ParsedInput *productenrich.ParsedInput     `json:"parsed_input,omitempty" gorm:"-"`
	Analysis    *productenrich.ProductAnalysis `json:"analysis,omitempty" gorm:"-"`
}

type ImageAudit struct {
	ImageURL          string   `json:"image_url"`
	IsWhiteBackground bool     `json:"is_white_background"`
	HasOverlayText    bool     `json:"has_overlay_text"`
	HasPromoBadge     bool     `json:"has_promo_badge"`
	HasLogo           bool     `json:"has_logo"`
	IsCollage         bool     `json:"is_collage"`
	SharpnessScore    float64  `json:"sharpness_score"`
	QualityScore      float64  `json:"quality_score"`
	PrimaryObject     string   `json:"primary_object,omitempty"`
	Issues            []string `json:"issues,omitempty"`
}

type ImageStageTrace struct {
	Stage      string `json:"stage"`
	ImageURL   string `json:"image_url,omitempty"`
	AssetType  string `json:"asset_type,omitempty"`
	Outcome    string `json:"outcome"`
	DurationMS int64  `json:"duration_ms"`
	Message    string `json:"message,omitempty"`
}

type ImageStageSummary struct {
	Stage      string `json:"stage"`
	Outcome    string `json:"outcome"`
	DurationMS int64  `json:"duration_ms"`
	Message    string `json:"message,omitempty"`
}

type ImageCandidateSet struct {
	PrimarySource   string   `json:"primary_source,omitempty"`
	HeroCandidates  []string `json:"hero_candidates,omitempty"`
	SceneCandidates []string `json:"scene_candidates,omitempty"`
	RejectedImages  []string `json:"rejected_images,omitempty"`
}

type ImageAsset struct {
	URL        string            `json:"url"`
	Type       AssetType         `json:"type"`
	SourceURL  string            `json:"source_url,omitempty"`
	Operations []string          `json:"operations,omitempty"`
	Width      int               `json:"width,omitempty"`
	Height     int               `json:"height,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type ImageIssue struct {
	ImageURL string `json:"image_url"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

type ComplianceReport struct {
	Marketplace string       `json:"marketplace"`
	Passed      bool         `json:"passed"`
	Issues      []ImageIssue `json:"issues,omitempty"`
}

type QualityAssessment struct {
	OverallScore float64  `json:"overall_score"`
	MainScore    float64  `json:"main_score"`
	WhiteBgScore float64  `json:"white_bg_score"`
	Issues       []string `json:"issues,omitempty"`
}

type ReviewDecision struct {
	NeedsReview bool     `json:"needs_review"`
	Reasons     []string `json:"reasons,omitempty"`
}

type ImageProcessResult struct {
	MainImage      *ImageAsset         `json:"main_image,omitempty"`
	WhiteBgImage   *ImageAsset         `json:"white_bg_image,omitempty"`
	SubjectCutout  *ImageAsset         `json:"subject_cutout,omitempty"`
	GalleryImages  []ImageAsset        `json:"gallery_images,omitempty"`
	RejectedImages []ImageIssue        `json:"rejected_images,omitempty"`
	Compliance     *ComplianceReport   `json:"compliance,omitempty"`
	Quality        *QualityAssessment  `json:"quality,omitempty"`
	Review         *ReviewDecision     `json:"review,omitempty"`
	StageSummaries []ImageStageSummary `json:"stage_summaries,omitempty"`
	ImageTraces    []ImageStageTrace   `json:"image_traces,omitempty"`
}

type TaskResult struct {
	TaskID      string              `json:"task_id"`
	Status      TaskStatus          `json:"status"`
	Result      *ImageProcessResult `json:"result,omitempty"`
	Error       string              `json:"error,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
	CompletedAt *time.Time          `json:"completed_at,omitempty"`
}

func (r ImageProcessRequest) Value() (driver.Value, error) {
	return json.Marshal(r)
}

func (r *ImageProcessRequest) Scan(value any) error {
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

func (r ImageProcessResult) Value() (driver.Value, error) {
	return json.Marshal(r)
}

func (r *ImageProcessResult) Scan(value any) error {
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
