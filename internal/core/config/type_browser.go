package config

import "time"

// BrowserConfig 浏览器通用配置
type BrowserConfig struct {
	Enabled        bool                `yaml:"enabled"`
	Headless       bool                `yaml:"headless"`
	BrowserPath    string              `yaml:"browserPath"`
	UserDataDir    string              `yaml:"userDataDir"`
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
	MaxUsesPerInstance  int    `yaml:"maxUsesPerInstance"`  // 单实例最大复用次数，达到后轮换 context
}

// BrowserCollectorConfig wires the current-application to the standalone
// anonymous public 1688 browser collector process (architecture design D13).
//
// The collector is opt-in: with an empty Endpoint or an empty Credential the
// application keeps the existing anonymous public HTTP provider unchanged. The
// Credential is a service-to-service credential used only between the
// application and the collector; it is never a user or tenant credential and
// must not be reused from any database, session, or user store.
type BrowserCollectorConfig struct {
	// Endpoint is the collector base URL, for example
	// "http://127.0.0.1:19545". Empty disables the collector path.
	Endpoint string `yaml:"endpoint"`
	// Credential is the shared service credential presented to the collector.
	// Empty disables the collector path.
	Credential string `yaml:"credential"`
	// Timeout bounds one collector RPC call. Zero means the default acquisition
	// timeout.
	Timeout time.Duration `yaml:"timeout"`
}
