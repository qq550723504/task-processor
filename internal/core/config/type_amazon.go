package config

// AmazonConfig Amazon爬虫配置
type AmazonConfig struct {
	Enabled           bool              `yaml:"enabled"`
	Zipcodes          map[string]string `yaml:"zipcodes"`
	DataFreshnessDays int               `yaml:"dataFreshnessDays"`
	CrawlTimeout      int               `yaml:"crawlTimeout"` // 爬虫超时时间（秒）
	SPAPI             SPAPIConfig       `yaml:"spapi"`
	ConfigPaths       AmazonConfigPaths `yaml:"configPaths"` // 业务配置文件路径（统一管理）

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
