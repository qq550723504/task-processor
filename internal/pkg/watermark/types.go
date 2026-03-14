// Package watermark 提供图片水印检测与去除功能
package watermark

import (
	"image"
)

// DetectionMethod 检测方法
type DetectionMethod string

const (
	// DetectionMethodTraditional 传统图像处理算法
	DetectionMethodTraditional DetectionMethod = "traditional"
	// DetectionMethodAI AI视觉模型检测
	DetectionMethodAI DetectionMethod = "ai"
	// DetectionMethodHybrid 混合方案（传统+AI）
	DetectionMethodHybrid DetectionMethod = "hybrid"
)

// RemovalMethod 去除方法
type RemovalMethod string

const (
	// RemovalMethodInpaint 图像修复算法
	RemovalMethodInpaint RemovalMethod = "inpaint"
	// RemovalMethodBlur 模糊处理
	RemovalMethodBlur RemovalMethod = "blur"
	// RemovalMethodCrop 裁剪处理
	RemovalMethodCrop RemovalMethod = "crop"
	// RemovalMethodAI AI模型去除（如LaMa）
	RemovalMethodAI RemovalMethod = "ai"
)

// WatermarkType 水印类型
type WatermarkType string

const (
	WatermarkTypeText    WatermarkType = "text"    // 文字水印
	WatermarkTypeLogo    WatermarkType = "logo"    // Logo水印
	WatermarkTypePattern WatermarkType = "pattern" // 图案水印
	WatermarkTypeUnknown WatermarkType = "unknown" // 未知类型
)

// Position 水印位置
type Position string

const (
	PositionTopLeft     Position = "top_left"
	PositionTopRight    Position = "top_right"
	PositionBottomLeft  Position = "bottom_left"
	PositionBottomRight Position = "bottom_right"
	PositionCenter      Position = "center"
	PositionCustom      Position = "custom"
)

// WatermarkRegion 水印区域
type WatermarkRegion struct {
	// 区域坐标
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`

	// 水印信息
	Type       WatermarkType `json:"type"`
	Position   Position      `json:"position"`
	Confidence float64       `json:"confidence"` // 置信度 0-1

	// 额外信息
	Description string                 `json:"description,omitempty"` // AI检测时的描述
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// DetectionResult 检测结果
type DetectionResult struct {
	HasWatermark bool               `json:"has_watermark"`
	Regions      []*WatermarkRegion `json:"regions"`
	Method       DetectionMethod    `json:"method"`
	ProcessTime  float64            `json:"process_time"` // 处理时间（秒）
	Error        string             `json:"error,omitempty"`
}

// RemovalResult 去除结果
type RemovalResult struct {
	Success     bool                   `json:"success"`
	Image       image.Image            `json:"-"` // 处理后的图片
	Method      RemovalMethod          `json:"method"`
	ProcessTime float64                `json:"process_time"` // 处理时间（秒）
	Quality     float64                `json:"quality"`      // 处理质量评分 0-1
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ProcessResult 完整处理结果
type ProcessResult struct {
	Detection *DetectionResult `json:"detection"`
	Removal   *RemovalResult   `json:"removal,omitempty"`
	Original  image.Image      `json:"-"` // 原始图片
}

// Config 水印处理配置
type Config struct {
	// 全局开关
	Enabled bool `yaml:"enabled" json:"enabled"`

	// 检测配置
	Detection DetectionConfig `yaml:"detection" json:"detection"`

	// 去除配置
	Removal RemovalConfig `yaml:"removal" json:"removal"`

	// AI配置
	AI AIConfig `yaml:"ai" json:"ai"`

	// 性能配置
	Performance PerformanceConfig `yaml:"performance" json:"performance"`
}

// DetectionConfig 检测配置
type DetectionConfig struct {
	Method      DetectionMethod `yaml:"method" json:"method"`           // traditional/ai/hybrid
	Sensitivity string          `yaml:"sensitivity" json:"sensitivity"` // low/medium/high
	Regions     []string        `yaml:"regions" json:"regions"`         // corner/center/edge/full
	MinSize     int             `yaml:"min_size" json:"min_size"`       // 最小水印尺寸（像素）
	Threshold   float64         `yaml:"threshold" json:"threshold"`     // 检测阈值 0-1
}

// RemovalConfig 去除配置
type RemovalConfig struct {
	Method         RemovalMethod `yaml:"method" json:"method"`                   // inpaint/blur/crop/ai
	Quality        string        `yaml:"quality" json:"quality"`                 // low/medium/high
	PreserveAspect bool          `yaml:"preserve_aspect" json:"preserve_aspect"` // 保持宽高比
	AutoCrop       bool          `yaml:"auto_crop" json:"auto_crop"`             // 自动裁剪边缘水印
	BlurRadius     int           `yaml:"blur_radius" json:"blur_radius"`         // 模糊半径
	InpaintRadius  int           `yaml:"inpaint_radius" json:"inpaint_radius"`   // 修复半径
}

// AIConfig AI配置
type AIConfig struct {
	// GPT-4 Vision / Claude配置
	VisionAPI VisionAPIConfig `yaml:"vision_api" json:"vision_api"`

	// LaMa模型配置
	LamaModel LamaModelConfig `yaml:"lama_model" json:"lama_model"`

	// 商业API配置
	CommercialAPI CommercialAPIConfig `yaml:"commercial_api" json:"commercial_api"`
}

// VisionAPIConfig 视觉API配置
type VisionAPIConfig struct {
	Enabled  bool    `yaml:"enabled" json:"enabled"`
	Provider string  `yaml:"provider" json:"provider"` // openai/anthropic
	APIKey   string  `yaml:"api_key" json:"api_key"`
	Model    string  `yaml:"model" json:"model"` // gpt-4-vision-preview/claude-3-opus
	BaseURL  string  `yaml:"base_url" json:"base_url"`
	MaxCost  float64 `yaml:"max_cost" json:"max_cost"` // 单张图片最大成本（美元）
}

// LamaModelConfig LaMa模型配置
type LamaModelConfig struct {
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	ModelPath string `yaml:"model_path" json:"model_path"` // 本地模型路径
	ServerURL string `yaml:"server_url" json:"server_url"` // 远程服务URL
	UseGPU    bool   `yaml:"use_gpu" json:"use_gpu"`
	BatchSize int    `yaml:"batch_size" json:"batch_size"`
}

// CommercialAPIConfig 商业API配置
type CommercialAPIConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Provider string `yaml:"provider" json:"provider"` // cloudinary/removebg
	APIKey   string `yaml:"api_key" json:"api_key"`
	Endpoint string `yaml:"endpoint" json:"endpoint"`
}

// PerformanceConfig 性能配置
type PerformanceConfig struct {
	MaxConcurrent int     `yaml:"max_concurrent" json:"max_concurrent"` // 最大并发数
	Timeout       int     `yaml:"timeout" json:"timeout"`               // 超时时间（秒）
	CacheEnabled  bool    `yaml:"cache_enabled" json:"cache_enabled"`   // 启用缓存
	CacheTTL      int     `yaml:"cache_ttl" json:"cache_ttl"`           // 缓存过期时间（秒）
	MaxImageSize  int     `yaml:"max_image_size" json:"max_image_size"` // 最大图片尺寸（像素）
	QualityScore  float64 `yaml:"quality_score" json:"quality_score"`   // 质量评分阈值
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled: true,
		Detection: DetectionConfig{
			Method:      DetectionMethodHybrid,
			Sensitivity: "medium",
			Regions:     []string{"corner", "edge"},
			MinSize:     20,
			Threshold:   0.6,
		},
		Removal: RemovalConfig{
			Method:         RemovalMethodInpaint,
			Quality:        "high",
			PreserveAspect: true,
			AutoCrop:       true,
			BlurRadius:     5,
			InpaintRadius:  10,
		},
		AI: AIConfig{
			VisionAPI: VisionAPIConfig{
				Enabled:  false,
				Provider: "openai",
				Model:    "gpt-4-vision-preview",
				MaxCost:  0.05,
			},
			LamaModel: LamaModelConfig{
				Enabled:   false,
				UseGPU:    false,
				BatchSize: 1,
			},
			CommercialAPI: CommercialAPIConfig{
				Enabled: false,
			},
		},
		Performance: PerformanceConfig{
			MaxConcurrent: 3,
			Timeout:       30,
			CacheEnabled:  true,
			CacheTTL:      3600,
			MaxImageSize:  4096,
			QualityScore:  0.8,
		},
	}
}
