// Package config 提供配置管理功能
package config

// ProcessorConfig 处理器配置
type ProcessorConfig struct {
	MaxRetries int `yaml:"maxRetries"` // 最大重试次数
	Timeout    int `yaml:"timeout"`    // 超时时间（秒）
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

// PlatformsConfig 平台配置
type PlatformsConfig struct {
	Temu  PlatformConfig `yaml:"temu"`  // TEMU平台配置
	Shein PlatformConfig `yaml:"shein"` // SHEIN平台配置
}

// PlatformConfig 单个平台的完整配置
type PlatformConfig struct {
	Enabled     bool              `yaml:"enabled"`     // 是否启用该平台处理器
	AutoPricing AutoPricingConfig `yaml:"autoPricing"` // 自动定价配置
	Sync        SyncConfig        `yaml:"sync"`        // 产品同步配置
	Monitor     MonitorConfig     `yaml:"monitor"`     // 产品监控配置
}

// AutoPricingConfig 自动定价配置
type AutoPricingConfig struct {
	Enabled   bool `yaml:"enabled"`   // 是否启用自动定价
	Interval  int  `yaml:"interval"`  // 定价间隔（秒）
	BatchSize int  `yaml:"batchSize"` // 批量处理大小
}

// AmazonConfig Amazon爬虫配置
type AmazonConfig struct {
	Enabled           bool              `yaml:"enabled"`
	Headless          bool              `yaml:"headless"`
	BrowserPath       string            `yaml:"browserPath"`
	PoolSize          int               `yaml:"poolSize"` // 浏览器池大小，同时也是并发处理数
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
	Enabled   bool    `yaml:"enabled"`   // 是否启用产品同步
	StoreIDs  []int64 `yaml:"storeIDs"`  // 需要同步的店铺ID列表
	Interval  int     `yaml:"interval"`  // 同步间隔（分钟）
	BatchSize int     `yaml:"batchSize"` // 批量处理大小
}

// MonitorConfig 产品监控配置
type MonitorConfig struct {
	Enabled              bool    `yaml:"enabled"`              // 是否启用产品监控
	StoreIDs             []int64 `yaml:"storeIDs"`             // 需要监控的店铺ID列表
	CheckInterval        int     `yaml:"checkInterval"`        // 检查间隔（分钟）
	BatchSize            int     `yaml:"batchSize"`            // 批量处理大小
	EnablePriceAlert     bool    `yaml:"enablePriceAlert"`     // 启用价格告警
	EnableStockAlert     bool    `yaml:"enableStockAlert"`     // 启用库存告警
	PriceChangeThreshold float64 `yaml:"priceChangeThreshold"` // 价格变化阈值（百分比）
	StockChangeThreshold int     `yaml:"stockChangeThreshold"` // 库存变化阈值
}
