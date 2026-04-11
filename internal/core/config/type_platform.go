package config

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
	FetchMode            string              `yaml:"fetchMode"`            // 商品抓取模式：auto/local/distributed/remote-api
	AutoPricing          AutoPricingConfig   `yaml:"autoPricing"`          // 自动核价配置
	ProductSync          ScheduledTaskConfig `yaml:"productSync"`          // 产品同步配置
	InventorySync        ScheduledTaskConfig `yaml:"inventorySync"`        // 库存同步配置
	ActivityRegistration ScheduledTaskConfig `yaml:"activityRegistration"` // 活动报名配置
	SyncProduct          SyncProductConfig   `yaml:"sync"`                 // 旧版产品同步配置，仅保留兼容
	Monitor              MonitorConfig       `yaml:"monitor"`              // 产品监控配置
	ConfigPaths          PlatformConfigPaths `yaml:"configPaths"`          // 业务配置文件路径（统一管理）
}

// PlatformConfigPaths 平台业务配置文件路径
type PlatformConfigPaths struct {
	SensitiveWords   string `yaml:"sensitiveWords"`   // 敏感词配置文件路径
	ProhibitedItems  string `yaml:"prohibitedItems"`  // 违禁品配置文件路径
	AttributeMapping string `yaml:"attributeMapping"` // 属性映射配置文件路径
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

// SyncProductConfig 旧版产品同步配置
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
