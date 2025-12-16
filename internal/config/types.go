// Package config 提供配置管理功能
package config

// ProcessorConfig 处理器配置
type ProcessorConfig struct {
	MaxRetries int
	Timeout    int // 单位：秒
}

// WorkerConfig 工作池配置
type WorkerConfig struct {
	Concurrency      int // 并发工作协程数
	BufferSize       int // 队列缓冲区大小
	TaskInterval     int // 任务获取间隔（秒）
	MaxFetchPerCycle int // 单次最多获取任务数
	QueueThreshold   int // 队列使用率阈值（%）
}

// OpenAIConfig OpenAI配置
type OpenAIConfig struct {
	APIKey  string
	Model   string
	BaseURL string
	Timeout int // 单位：秒
}

// ManagementConfig 管理系统配置
type ManagementConfig struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	TokenURL     string
	Scopes       []string
	TenantID     string // 租户ID
	UserID       int64
	StoreIDs     []int64
}

// AutoPricingConfig 自动核价配置
type AutoPricingConfig struct {
	Temu  PlatformAutoPricingConfig
	Shein PlatformAutoPricingConfig
}

// PlatformAutoPricingConfig 平台自动核价配置
type PlatformAutoPricingConfig struct {
	Enabled   bool
	Interval  int
	BatchSize int
}

// AmazonConfig Amazon爬虫配置
type AmazonConfig struct {
	Enabled           bool              `yaml:"enabled"`
	Headless          bool              `yaml:"headless"`
	BrowserPath       string            `yaml:"browserPath"`
	PoolSize          int               `yaml:"poolSize"`
	Zipcodes          map[string]string `yaml:"zipcodes"`
	ViewportWidth     int               `yaml:"viewportWidth"`
	ViewportHeight    int               `yaml:"viewportHeight"`
	ProxyServer       string            `yaml:"proxyServer"`
	DataFreshnessDays int               `yaml:"dataFreshnessDays"`
	SPAPI             SPAPIConfig       `yaml:"spapi"`
}

// MarketplaceConfig 单个市场配置
type MarketplaceConfig struct {
	MarketplaceID string `yaml:"marketplaceID"`
	SellerID      string `yaml:"sellerID"`
	Currency      string `yaml:"currency"`
	Name          string `yaml:"name"`
	Enabled       bool   `yaml:"enabled"`
}

// SPAPIConfig Amazon SP-API配置
type SPAPIConfig struct {
	Enabled                bool                         `yaml:"enabled"`
	Region                 string                       `yaml:"region"`
	DefaultMarketplace     string                       `yaml:"defaultMarketplace"`
	Marketplaces           map[string]MarketplaceConfig `yaml:"markets"`
	ClientID               string                       `yaml:"clientID"`
	ClientSecret           string                       `yaml:"clientSecret"`
	RefreshToken           string                       `yaml:"refreshToken"`
	AWSAccessKeyID         string                       `yaml:"awsAccessKeyID"`
	AWSSecretKey           string                       `yaml:"awsSecretKey"`
	DefaultFulfillmentType string                       `yaml:"defaultFulfillmentType"`
	DefaultCondition       string                       `yaml:"defaultCondition"`

	// 向后兼容字段（已废弃）
	MarketplaceID string `yaml:"marketplaceID,omitempty"`
	SellerID      string `yaml:"sellerID,omitempty"`
}

// UpdaterConfig 自动更新配置
type UpdaterConfig struct {
	Enabled            bool   `yaml:"enabled"`              // 是否启用自动更新
	UpdateURL          string `yaml:"update_url"`           // 版本检查地址
	CheckInterval      int    `yaml:"check_interval"`       // 检查间隔（秒）
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"` // 跳过TLS证书验证
	CurrentVersion     string `yaml:"-"`                    // 当前版本（从编译时注入）
}

// SyncConfig 产品同步配置
type SyncConfig struct {
	Enabled  bool    `yaml:"enabled"`   // 是否启用产品同步
	StoreIDs []int64 `yaml:"store_ids"` // 需要同步的店铺ID列表
}

// MonitorConfig 产品监控配置
type MonitorConfig struct {
	Enabled              bool    `yaml:"enabled"`                // 是否启用产品监控
	StoreIDs             []int64 `yaml:"store_ids"`              // 需要监控的店铺ID列表
	CheckInterval        int     `yaml:"check_interval"`         // 检查间隔（分钟）
	BatchSize            int     `yaml:"batch_size"`             // 批量处理大小
	EnablePriceAlert     bool    `yaml:"enable_price_alert"`     // 启用价格告警
	EnableStockAlert     bool    `yaml:"enable_stock_alert"`     // 启用库存告警
	PriceChangeThreshold float64 `yaml:"price_change_threshold"` // 价格变化阈值（百分比）
	StockChangeThreshold int     `yaml:"stock_change_threshold"` // 库存变化阈值
}

// PlatformConfig 平台特定配置
type PlatformConfig struct {
	Name string // "temu" 或 "shein"
	Type string // "web" 或 "cli"
}
