// Package config 提供配置管理功能
package config

// ProcessorConfig 处理器配置
type ProcessorConfig struct {
	MaxRetries int `yaml:"maxRetries"` // 最大重试次数
	Timeout    int `yaml:"timeout"`    // 超时时间（秒）
}

// WorkerConfig 工作池配置
type WorkerConfig struct {
	Concurrency        int `yaml:"concurrency"`        // 并发工作协程数
	BufferSize         int `yaml:"bufferSize"`         // 队列缓冲区大小
	TaskInterval       int `yaml:"taskInterval"`       // 任务获取间隔（秒）
	MaxFetchPerCycle   int `yaml:"maxFetchPerCycle"`   // 单次最多获取任务数
	QueueThreshold     int `yaml:"queueThreshold"`     // 队列使用率阈值（%）
	CleanupInterval    int `yaml:"cleanupInterval"`    // 清理间隔（秒）
	TaskTimeout        int `yaml:"taskTimeout"`        // 任务超时时间（秒）
	StuckTaskThreshold int `yaml:"stuckTaskThreshold"` // 任务卡住阈值（秒）
	ForceCleanupAfter  int `yaml:"forceCleanupAfter"`  // 30分钟强制清理阈值（秒）
}

// OpenAIConfig OpenAI配置
type OpenAIConfig struct {
	APIKey  string `yaml:"apiKey"`  // API密钥
	Model   string `yaml:"model"`   // 模型名称
	BaseURL string `yaml:"baseURL"` // API基础URL
	Timeout int    `yaml:"timeout"` // 超时时间（秒）
}

// ManagementConfig 管理系统配置
type ManagementConfig struct {
	BaseURL      string   `yaml:"baseURL"`
	ClientID     string   `yaml:"clientID"`
	ClientSecret string   `yaml:"clientSecret"`
	TokenURL     string   `yaml:"tokenURL"`
	Scopes       []string `yaml:"scopes"`
	TenantID     string   `yaml:"tenantID"` // 租户ID
	UserID       int64    `yaml:"userID"`
	StoreIDs     []int64  `yaml:"storeIDs"`
}

// PlatformsConfig 平台配置
type PlatformsConfig struct {
	Temu        PlatformConfig    `yaml:"temu"`        // TEMU平台配置
	Shein       PlatformConfig    `yaml:"shein"`       // SHEIN平台配置
	Alibaba1688 Alibaba1688Config `yaml:"alibaba1688"` // 1688平台配置
}

// PlatformConfig 单个平台的完整配置
type PlatformConfig struct {
	Enabled              bool                `yaml:"enabled"`              // 是否启用该平台处理器（上架任务处理）
	SchedulerEnabled     bool                `yaml:"schedulerEnabled"`     // 是否启用调度任务（核价、同步等）
	AutoPricing          AutoPricingConfig   `yaml:"autoPricing"`          // 自动核价配置
	ProductSync          ScheduledTaskConfig `yaml:"productSync"`          // 产品同步配置
	InventorySync        ScheduledTaskConfig `yaml:"inventorySync"`        // 库存同步配置
	ActivityRegistration ScheduledTaskConfig `yaml:"activityRegistration"` // 活动报名配置
	SyncProduct          SyncProductConfig   `yaml:"sync"`                 // 产品同步配置（旧版，保留兼容）
	Monitor              MonitorConfig       `yaml:"monitor"`              // 产品监控配置
}

// AutoPricingConfig 自动定价配置
type AutoPricingConfig struct {
	Enabled        bool `yaml:"enabled"`        // 是否启用自动定价
	Interval       int  `yaml:"interval"`       // 定价间隔（秒）
	BatchSize      int  `yaml:"batchSize"`      // 批量处理大小
	UseAmazonPrice bool `yaml:"useAmazonPrice"` // 是否使用Amazon价格数据进行定价决策
}

// ScheduledTaskConfig 调度任务配置
type ScheduledTaskConfig struct {
	Enabled  bool `yaml:"enabled"`  // 是否启用
	Interval int  `yaml:"interval"` // 执行间隔（秒）
}

// BrowserConfig 浏览器通用配置
type BrowserConfig struct {
	Enabled        bool                `yaml:"enabled"`
	Headless       bool                `yaml:"headless"`
	BrowserPath    string              `yaml:"browserPath"`
	PoolSize       int                 `yaml:"poolSize"` // 浏览器池大小
	ViewportWidth  int                 `yaml:"viewportWidth"`
	ViewportHeight int                 `yaml:"viewportHeight"`
	ProxyServer    string              `yaml:"proxyServer"`
	RandomConfig   BrowserRandomConfig `yaml:"randomConfig"` // 随机配置选项
}

// BrowserRandomConfig 浏览器随机配置
type BrowserRandomConfig struct {
	Enabled             bool   `yaml:"enabled"`             // 是否启用随机配置
	Strategy            string `yaml:"strategy"`            // 配置策略: random, stable, preset, windows
	PresetName          string `yaml:"presetName"`          // 预设名称（当strategy为preset时使用）
	FingerprintStrategy string `yaml:"fingerprintStrategy"` // 指纹策略: random, stable
	HealthCheckEnabled  bool   `yaml:"healthCheckEnabled"`  // 是否启用健康检查
	MaxRetries          int    `yaml:"maxRetries"`          // 最大重试次数
}

// AmazonConfig Amazon爬虫配置
type AmazonConfig struct {
	Enabled           bool              `yaml:"enabled"`
	Zipcodes          map[string]string `yaml:"zipcodes"`
	DataFreshnessDays int               `yaml:"dataFreshnessDays"`
	CrawlTimeout      int               `yaml:"crawlTimeout"` // 爬虫超时时间（秒）
	SPAPI             SPAPIConfig       `yaml:"spapi"`

	// 临时兼容性字段（从 Browser 配置继承）
	Headless       bool                `yaml:"-"` // 不序列化，从 Browser 配置获取
	BrowserPath    string              `yaml:"-"` // 不序列化，从 Browser 配置获取
	PoolSize       int                 `yaml:"-"` // 不序列化，从 Browser 配置获取
	ViewportWidth  int                 `yaml:"-"` // 不序列化，从 Browser 配置获取
	ViewportHeight int                 `yaml:"-"` // 不序列化，从 Browser 配置获取
	ProxyServer    string              `yaml:"-"` // 不序列化，从 Browser 配置获取
	RandomConfig   BrowserRandomConfig `yaml:"-"` // 不序列化，从 Browser 配置获取
}

// RabbitMQConfig RabbitMQ配置
type RabbitMQConfig struct {
	Enabled           bool   `yaml:"enabled"`           // 是否启用RabbitMQ分布式爬虫
	URL               string `yaml:"url"`               // RabbitMQ连接URL
	ReconnectInterval int    `yaml:"reconnectInterval"` // 重连间隔（秒）
	MaxReconnectTries int    `yaml:"maxReconnectTries"` // 最大重连次数
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
type SyncProductConfig struct {
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

// Alibaba1688Config 1688平台配置
type Alibaba1688Config struct {
	Enabled  bool `yaml:"enabled"`  // 是否启用1688处理器
	Timeout  int  `yaml:"timeout"`  // 处理超时时间（秒）
	PoolSize int  `yaml:"poolSize"` // 浏览器池大小
}
