package config

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
