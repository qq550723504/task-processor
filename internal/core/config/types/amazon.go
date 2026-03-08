// Package types 提供配置类型定义
package types

// AmazonConfig Amazon爬虫配置
type AmazonConfig struct {
	Enabled           bool              `yaml:"enabled"`
	Zipcodes          map[string]string `yaml:"zipcodes"`
	DataFreshnessDays int               `yaml:"dataFreshnessDays"`
	CrawlTimeout      int               `yaml:"crawlTimeout"` // 爬虫超时时间（秒）
	SPAPI             SPAPIConfig       `yaml:"spapi"`
	ConfigPaths       AmazonConfigPaths `yaml:"configPaths"` // 业务配置文件路径（统一管理）

	// 临时兼容性字段（从 Browser 配置继承）
	Headless       bool                `yaml:"-"` // 不序列化，从 Browser 配置获取
	BrowserPath    string              `yaml:"-"` // 不序列化，从 Browser 配置获取
	PoolSize       int                 `yaml:"-"` // 不序列化，从 Browser 配置获取
	ViewportWidth  int                 `yaml:"-"` // 不序列化，从 Browser 配置获取
	ViewportHeight int                 `yaml:"-"` // 不序列化，从 Browser 配置获取
	ProxyServer    string              `yaml:"-"` // 不序列化，从 Browser 配置获取
	RandomConfig   BrowserRandomConfig `yaml:"-"` // 不序列化，从 Browser 配置获取
}

// AmazonConfigPaths Amazon业务配置文件路径
type AmazonConfigPaths struct {
	AttributeMapping string `yaml:"attributeMapping"` // 属性映射配置文件路径
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
