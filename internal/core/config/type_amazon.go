package config

// AmazonConfig Amazon爬虫配置
type AmazonConfig struct {
	Enabled            bool                           `yaml:"enabled"`
	Zipcodes           map[string]string              `yaml:"zipcodes"`
	DataFreshnessDays  int                            `yaml:"dataFreshnessDays"`
	CrawlTimeout       int                            `yaml:"crawlTimeout"` // 爬虫超时时间（秒）
	ProductDedupe      ProductDedupeConfig            `yaml:"productDedupe"`
	FailureArtifacts   FailureArtifactsConfig         `yaml:"failureArtifacts"`
	RiskControl        AmazonRiskControlConfig        `yaml:"riskControl"`
	RegionGuard        AmazonRegionGuardConfig        `yaml:"regionGuard"`
	QualityControl     AmazonQualityControlConfig     `yaml:"qualityControl"`
	ProxyPool          AmazonProxyPoolConfig          `yaml:"proxyPool"`
	ConcurrencyControl AmazonConcurrencyControlConfig `yaml:"concurrencyControl"`
	RemoteAPI          RemoteAPIConfig                `yaml:"remoteAPI"`
	SPAPI              SPAPIConfig                    `yaml:"spapi"`
	ConfigPaths        AmazonConfigPaths              `yaml:"configPaths"` // 业务配置文件路径（统一管理）

}

type ProductDedupeConfig struct {
	LockTTLSeconds     int `yaml:"lockTTLSeconds"`
	ResultTTLSeconds   int `yaml:"resultTTLSeconds"`
	WaitTimeoutSeconds int `yaml:"waitTimeoutSeconds"`
	PollIntervalMillis int `yaml:"pollIntervalMillis"`
}

type RemoteAPIConfig struct {
	Enabled bool   `yaml:"enabled"`
	BaseURL string `yaml:"baseURL"`
	Timeout int    `yaml:"timeout"`
}

type FailureArtifactsConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Directory    string `yaml:"directory"`
	CaptureHTML  bool   `yaml:"captureHTML"`
	MaxHTMLBytes int    `yaml:"maxHTMLBytes"`
}

type AmazonRiskControlConfig struct {
	CaptchaRecreateThreshold        int `yaml:"captchaRecreateThreshold"`
	AuthenticationRecreateThreshold int `yaml:"authenticationRecreateThreshold"`
	BrowserCrashRecreateThreshold   int `yaml:"browserCrashRecreateThreshold"`
	TimeoutRecreateThreshold        int `yaml:"timeoutRecreateThreshold"`
	NetworkRecreateThreshold        int `yaml:"networkRecreateThreshold"`
	ServerErrorRecreateThreshold    int `yaml:"serverErrorRecreateThreshold"`
}

type AmazonRegionGuardConfig struct {
	Enabled                 bool `yaml:"enabled"`
	FailureThreshold        int  `yaml:"failureThreshold"`
	EvaluationWindowSeconds int  `yaml:"evaluationWindowSeconds"`
	CooldownSeconds         int  `yaml:"cooldownSeconds"`
}

type AmazonQualityControlConfig struct {
	RetryOnValidationFailure   bool `yaml:"retryOnValidationFailure"`
	ValidationRetryMaxAttempts int  `yaml:"validationRetryMaxAttempts"`
}

type AmazonProxyPoolConfig struct {
	Enabled                bool     `yaml:"enabled"`
	Strategy               string   `yaml:"strategy"`
	FailureCooldownSeconds int      `yaml:"failureCooldownSeconds"`
	Proxies                []string `yaml:"proxies"`
}

type AmazonConcurrencyControlConfig struct {
	Enabled               bool           `yaml:"enabled"`
	MaxInFlight           int            `yaml:"maxInFlight"`
	MaxWaiting            int            `yaml:"maxWaiting"`
	AcquireTimeoutSeconds int            `yaml:"acquireTimeoutSeconds"`
	PerRegion             map[string]int `yaml:"perRegion"`
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
