package productenrich

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"task-processor/internal/catalog/canonical"
	"time"
)

// ErrTaskNotFound 任务不存在错误
var ErrTaskNotFound = errors.New("task not found")

// ErrTaskNotPending 表示任务已被其他执行器抢占或已不再处于待处理状态。
var ErrTaskNotPending = errors.New("task is not pending")

// TaskStatus 表示任务的状态
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
)

// GenerateRequest 表示产品生成请求
type GenerateRequest struct {
	ImageURLs  []string `json:"image_urls" binding:"omitempty,dive,url"`
	Text       string   `json:"text" binding:"omitempty"`
	ProductURL string   `json:"product_url" binding:"omitempty,url"`
}

// Task 表示一个产品生成任务
type Task struct {
	ID         string           `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Request    *GenerateRequest `json:"request" gorm:"type:text"`
	Status     TaskStatus       `json:"status" gorm:"type:varchar(20);index"`
	Result     *ProductJSON     `json:"result,omitempty" gorm:"type:text"`
	Error      string           `json:"error,omitempty" gorm:"type:text"`
	CreatedAt  time.Time        `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time        `json:"updated_at" gorm:"autoUpdateTime"`
	RetryCount int              `json:"retry_count" gorm:"default:0"`
}

// ProductJSON 表示最终生成的产品 JSON 数据
type ProductJSON struct {
	Title             string                       `json:"title"`
	Category          []string                     `json:"category"`
	Attributes        map[string]string            `json:"attributes"`
	Specifications    *ProductSpecs                `json:"specifications"`
	VariantDimensions []ScrapedVariantDimension    `json:"variant_dimensions,omitempty"`
	Variants          []ProductVariant             `json:"variants"`
	SellingPoints     []string                     `json:"selling_points"`
	SEOKeywords       []string                     `json:"seo_keywords"`
	Description       string                       `json:"description"`
	Images            []string                     `json:"images"`
	Evidence          map[string][]CanonicalSource `json:"evidence,omitempty"`
	QualityScoring    *QualityScoringMetadata      `json:"quality_scoring,omitempty"`
}

// ProductSpecs 产品规格信息
type ProductSpecs = canonical.ProductSpecs

// Dimensions 尺寸信息
type Dimensions = canonical.Dimensions

// Weight 重量信息
type Weight = canonical.Weight

// PackageInfo 包装信息
type PackageInfo = canonical.PackageInfo

// ProductVariant 产品变体（SKU）
type ProductVariant struct {
	SKU        string            `json:"sku"`
	Attributes map[string]string `json:"attributes"`
	Price      *PriceInfo        `json:"price,omitempty"`
	Stock      int               `json:"stock"`
	Images     []string          `json:"images,omitempty"`
	Barcode    string            `json:"barcode,omitempty"`
	IsDefault  bool              `json:"is_default,omitempty"`
}

// PriceInfo 价格信息
type PriceInfo = canonical.PriceInfo

// ParsedInput 解析后的输入数据
type ParsedInput struct {
	Images      []string     `json:"images"`
	Text        string       `json:"text"`
	ScrapedData *ScrapedData `json:"scraped_data,omitempty"`
}

// ScrapedData 网页抓取的数据
type ScrapedData struct {
	Title             string                    `json:"title"`
	Category          string                    `json:"category,omitempty"`
	Description       string                    `json:"description"`
	Images            []string                  `json:"images"`
	Price             float64                   `json:"price"`
	Specs             map[string]string         `json:"specs"`
	VariantDimensions []ScrapedVariantDimension `json:"variant_dimensions,omitempty"`
	Variants          []ProductVariant          `json:"variants,omitempty"`
}

// ScrapedVariantDimension 表示抓取侧提供的销售属性维度和值。
type ScrapedVariantDimension = canonical.ScrapedVariantDimension

// ProductAnalysis 产品分析结果
type ProductAnalysis struct {
	ImageAttributes *ImageAttributes       `json:"image_attributes,omitempty"`
	TextAttributes  *TextAttributes        `json:"text_attributes,omitempty"`
	Representation  *ProductRepresentation `json:"representation,omitempty"`
	ScrapedData     *ScrapedData           `json:"scraped_data,omitempty"`
}

// ImageAttributes 图片属性
type ImageAttributes struct {
	Color    string `json:"color"`
	Material string `json:"material"`
	Scene    string `json:"scene"`
	Usage    string `json:"usage"`
}

// TextAttributes 文本属性
type TextAttributes struct {
	Title         string            `json:"title"`
	Attributes    map[string]string `json:"attributes"`
	SellingPoints []string          `json:"selling_points"`
}

// ProductRepresentation 产品表示
type ProductRepresentation struct {
	ProductType string            `json:"product_type"`
	Attributes  map[string]string `json:"attributes"`
	Features    []string          `json:"features"`
}

// EnhancementSuggestion 改进建议
type EnhancementSuggestion struct {
	RequiredActions  []string `json:"required_actions"`
	OptionalActions  []string `json:"optional_actions"`
	EstimatedQuality string   `json:"estimated_quality"`
}

// TaskResult 任务结果
type TaskResult struct {
	TaskID      string       `json:"task_id"`
	Status      TaskStatus   `json:"status"`
	ProductJSON *ProductJSON `json:"product_json,omitempty"`
	Error       string       `json:"error,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
}

// ProcessingStrategy 处理策略类型
type ProcessingStrategy string

const (
	StrategyFull    ProcessingStrategy = "full"
	StrategyBasic   ProcessingStrategy = "basic"
	StrategyMinimal ProcessingStrategy = "minimal"
	StrategyReject  ProcessingStrategy = "reject"
)

// ValidationResult 验证结果
type ValidationResult struct {
	IsValid          bool
	QualityScore     float64
	Issues           []ValidationIssue
	ImageScore       float64
	TextScore        float64
	ScrapedScore     float64
	ImageScorePrompt *PromptObservability
	TextScorePrompt  *PromptObservability
	// 原始验证对象，供 QualityScorer 的 LLM 评分使用
	ImageValidation *ImageValidation
	TextValidation  *TextValidation
}

// PromptObservability 表示一次 prompt 解析和使用的来源信息。
type PromptObservability struct {
	PromptRef     string
	PromptKey     string
	PromptSource  string
	PromptVersion string
}

func (o *PromptObservability) Clone() *PromptObservability {
	if o == nil {
		return nil
	}
	cloned := *o
	return &cloned
}

// QualityScoringMetadata 表示运行时质量评分链的可观测元数据。
type QualityScoringMetadata struct {
	QualityScore     float64              `json:"quality_score,omitempty"`
	ImageScore       float64              `json:"image_score,omitempty"`
	TextScore        float64              `json:"text_score,omitempty"`
	ScrapedScore     float64              `json:"scraped_score,omitempty"`
	ImageScorePrompt *PromptObservability `json:"image_score_prompt,omitempty"`
	TextScorePrompt  *PromptObservability `json:"text_score_prompt,omitempty"`
}

// ValidationIssue 验证问题
type ValidationIssue struct {
	Field    string
	Severity string
	Message  string
	Code     string
}

// ImageInfo 图片信息（用于验证缓存）
type ImageInfo struct {
	URL     string `json:"url"`
	IsValid bool   `json:"is_valid"`
	Width   int    `json:"width,omitempty"`
	Height  int    `json:"height,omitempty"`
	Format  string `json:"format,omitempty"`
	Size    int64  `json:"size,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Value 实现 driver.Valuer 接口
func (r GenerateRequest) Value() (driver.Value, error) {
	return json.Marshal(r)
}

// Scan 实现 sql.Scanner 接口
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

// Value 实现 driver.Valuer 接口
func (p ProductJSON) Value() (driver.Value, error) {
	return json.Marshal(p)
}

// Scan 实现 sql.Scanner 接口
func (p *ProductJSON) Scan(value any) error {
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, p)
}

// ResultValidation 结果验证
type ResultValidation struct {
	IsValid           bool
	ImageConsistency  bool
	KeywordMatchScore float64
	CompletenessScore float64
	Issues            []ValidationIssue
}

// CompletenessReport 完整性报告
type CompletenessReport struct {
	RequiredFields  map[string]bool
	OptionalFields  map[string]bool
	MissingRequired []string
	MissingOptional []string
	Score           float64
}

// SeverityError 错误级别
const SeverityError = "error"

// SeverityWarning 警告级别
const SeverityWarning = "warning"

// ImageValidation 图片验证结果
type ImageValidation struct {
	TotalCount  int
	ValidCount  int
	ValidImages []ImageInfo
}

// TextValidation 文本验证结果
type TextValidation struct {
	Length      int
	HasKeywords bool
	Keywords    []string
	RawText     string // 原始文本，供 LLM 评分使用
}

// ScrapedDataValidation 抓取数据验证结果
type ScrapedDataValidation struct {
	HasTitle       bool
	HasDescription bool
	HasImages      bool
	HasSpecs       bool
	HasPrice       bool
	ImageCount     int
}

// StrategyDetails 策略详细信息
type StrategyDetails struct {
	Strategy          ProcessingStrategy `json:"strategy"`
	Description       string             `json:"description"`
	EnabledSteps      []string           `json:"enabled_steps"`
	DisabledSteps     []string           `json:"disabled_steps"`
	ExpectedQuality   string             `json:"expected_quality"`
	EstimatedCost     string             `json:"estimated_cost"`
	EstimatedDuration string             `json:"estimated_duration"`
}
