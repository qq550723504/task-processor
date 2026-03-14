// Package productjson 定义产品JSON生成的领域模型
package productjson

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// ErrTaskNotFound 任务不存在错误
var ErrTaskNotFound = errors.New("task not found")

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
	ID         string           `json:"id"`
	Request    *GenerateRequest `json:"request"`
	Status     TaskStatus       `json:"status"`
	Result     *ProductJSON     `json:"result,omitempty"`
	Error      string           `json:"error,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
	RetryCount int              `json:"retry_count"`
}

// ProductJSON 表示最终生成的产品 JSON 数据
type ProductJSON struct {
	Title          string            `json:"title"`
	Category       []string          `json:"category"`
	Attributes     map[string]string `json:"attributes"`
	Specifications *ProductSpecs     `json:"specifications"`
	Variants       []ProductVariant  `json:"variants"`
	SellingPoints  []string          `json:"selling_points"`
	SEOKeywords    []string          `json:"seo_keywords"`
	Description    string            `json:"description"`
	Images         []string          `json:"images"`
}

// ProductSpecs 产品规格信息
type ProductSpecs struct {
	Dimensions *Dimensions       `json:"dimensions,omitempty"`
	Weight     *Weight           `json:"weight,omitempty"`
	Package    *PackageInfo      `json:"package,omitempty"`
	Technical  map[string]string `json:"technical,omitempty"`
}

// Dimensions 尺寸信息
type Dimensions struct {
	Length float64 `json:"length"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Unit   string  `json:"unit"`
}

// Weight 重量信息
type Weight struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// PackageInfo 包装信息
type PackageInfo struct {
	Dimensions *Dimensions `json:"dimensions,omitempty"`
	Weight     *Weight     `json:"weight,omitempty"`
	Quantity   int         `json:"quantity"`
}

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
type PriceInfo struct {
	Currency     string  `json:"currency"`
	Amount       float64 `json:"amount"`
	CompareAt    float64 `json:"compare_at,omitempty"`
	CostPrice    float64 `json:"cost_price,omitempty"`
	WholesaleMin int     `json:"wholesale_min,omitempty"`
}

// ParsedInput 解析后的输入数据
type ParsedInput struct {
	Images      []string     `json:"images"`
	Text        string       `json:"text"`
	ScrapedData *ScrapedData `json:"scraped_data,omitempty"`
}

// ScrapedData 网页抓取的数据
type ScrapedData struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Images      []string          `json:"images"`
	Price       float64           `json:"price"`
	Specs       map[string]string `json:"specs"`
}

// ProductAnalysis 产品分析结果
type ProductAnalysis struct {
	ImageAttributes *ImageAttributes       `json:"image_attributes,omitempty"`
	TextAttributes  *TextAttributes        `json:"text_attributes,omitempty"`
	Representation  *ProductRepresentation `json:"representation,omitempty"`
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
	IsValid      bool
	QualityScore float64
	Issues       []ValidationIssue
	ImageScore   float64
	TextScore    float64
	ScrapedScore float64
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
func (r *GenerateRequest) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, r)
}

// Value 实现 driver.Valuer 接口
func (p ProductJSON) Value() (driver.Value, error) {
	return json.Marshal(p)
}

// Scan 实现 sql.Scanner 接口
func (p *ProductJSON) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, p)
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
