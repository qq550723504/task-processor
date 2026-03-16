// package productenrich 提供产品JSON生成的应用层实现
package productenrich

import (
	"time"

	coreconfig "task-processor/internal/core/config"
)

// Config 产品JSON生成器配置
type Config struct {
	Server     coreconfig.ServerConfig   `mapstructure:"server"`
	Database   coreconfig.DatabaseConfig `mapstructure:"database"`
	Redis      coreconfig.RedisConfig    `mapstructure:"redis"`
	LLM        LLMConfig                 `mapstructure:"llm"`
	Worker     WorkerConfig              `mapstructure:"worker"`
	Scraper    ScraperConfig             `mapstructure:"scraper"`
	Logging    coreconfig.LogConfig      `mapstructure:"logging"`
	Validation ValidationConfig          `mapstructure:"validation"`
}

// LLMConfig LLM配置
type LLMConfig struct {
	DefaultClient string                      `mapstructure:"default_client"`
	Clients       map[string]*LLMClientConfig `mapstructure:"clients"`
}

// LLMClientConfig 单个LLM客户端配置
type LLMClientConfig struct {
	Provider   string        `mapstructure:"provider"`
	APIKey     string        `mapstructure:"api_key"`
	Model      string        `mapstructure:"model"`
	Timeout    time.Duration `mapstructure:"timeout"`
	MaxRetries int           `mapstructure:"max_retries"`
}

// WorkerConfig Worker配置
type WorkerConfig struct {
	Concurrency int           `mapstructure:"concurrency"`
	QueueName   string        `mapstructure:"queue_name"`
	TaskTimeout time.Duration `mapstructure:"task_timeout"`
}

// ScraperConfig 网页抓取配置
type ScraperConfig struct {
	Timeout     time.Duration `mapstructure:"timeout"`
	MaxRetries  int           `mapstructure:"max_retries"`
	UserAgent   string        `mapstructure:"user_agent"`
	WaitTimeout time.Duration `mapstructure:"wait_timeout"`
}

// ValidationConfig 输入验证配置
type ValidationConfig struct {
	QualityWeights     QualityWeightsConfig     `mapstructure:"quality_weights"`
	StrategyThresholds StrategyThresholdsConfig `mapstructure:"strategy_thresholds"`
	ImageValidation    ImageValidationConfig    `mapstructure:"image_validation"`
	LLMScoring         LLMScoringConfig         `mapstructure:"llm_scoring"`
}

// QualityWeightsConfig 质量评分权重配置
type QualityWeightsConfig struct {
	Image   float64 `mapstructure:"image"`
	Text    float64 `mapstructure:"text"`
	Scraped float64 `mapstructure:"scraped"`
}

// StrategyThresholdsConfig 策略选择阈值配置
type StrategyThresholdsConfig struct {
	Full    float64 `mapstructure:"full"`
	Basic   float64 `mapstructure:"basic"`
	Minimal float64 `mapstructure:"minimal"`
}

// ImageValidationConfig 图片验证配置
type ImageValidationConfig struct {
	Timeout       time.Duration `mapstructure:"timeout"`
	MaxConcurrent int           `mapstructure:"max_concurrent"`
	EnableCache   bool          `mapstructure:"enable_cache"`
	CacheTTL      time.Duration `mapstructure:"cache_ttl"`
}

// LLMScoringConfig LLM智能评分配置
type LLMScoringConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	TextClient   string `mapstructure:"text_client"`
	VisionClient string `mapstructure:"vision_client"`
}
