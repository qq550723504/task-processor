package config

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
