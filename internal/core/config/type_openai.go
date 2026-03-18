package config

// OpenAIClientConfig 单个 OpenAI 客户端配置
type OpenAIClientConfig struct {
	APIKey  string `yaml:"apiKey"`
	Model   string `yaml:"model"`
	BaseURL string `yaml:"baseURL"`
	Timeout int    `yaml:"timeout"`
}

// OpenAIConfig OpenAI配置
type OpenAIConfig struct {
	APIKey  string `yaml:"apiKey"`  // 默认 API 密钥
	Model   string `yaml:"model"`   // 默认模型名称
	BaseURL string `yaml:"baseURL"` // 默认 API 基础 URL
	Timeout int    `yaml:"timeout"` // 默认超时时间（秒）

	// Clients 各阶段命名客户端，key 对应 LLMManager.GetClient(name)
	// 支持的 key：default, vision, fast, scorer
	Clients map[string]OpenAIClientConfig `yaml:"clients"`
}
