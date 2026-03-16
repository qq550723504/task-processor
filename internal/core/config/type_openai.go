package config

// OpenAIConfig OpenAI配置
type OpenAIConfig struct {
	APIKey  string `yaml:"apiKey"`  // API密钥
	Model   string `yaml:"model"`   // 模型名称
	BaseURL string `yaml:"baseURL"` // API基础URL
	Timeout int    `yaml:"timeout"` // 超时时间（秒）
}
