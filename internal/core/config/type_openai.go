package config

import openaiClient "task-processor/internal/infra/clients/openai"

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

// ToClientConfig 将 OpenAIConfig 转换为 openai.ClientConfig，方便各处统一使用。
func (c OpenAIConfig) ToClientConfig() *openaiClient.ClientConfig {
	return openaiClient.NewClientConfig(c.APIKey, c.Model, c.BaseURL, c.Timeout)
}

// ToClientConfigs 将 OpenAIConfig 展开为命名客户端 map，包含 "default" 及所有子客户端。
// 子客户端未配置的字段（APIKey/BaseURL/Timeout）自动继承顶层默认值。
func (c OpenAIConfig) ToClientConfigs() map[string]*openaiClient.ClientConfig {
	cfgs := map[string]*openaiClient.ClientConfig{
		"default": c.ToClientConfig(),
	}
	for name, sub := range c.Clients {
		apiKey := sub.APIKey
		if apiKey == "" {
			apiKey = c.APIKey
		}
		baseURL := sub.BaseURL
		if baseURL == "" {
			baseURL = c.BaseURL
		}
		timeout := sub.Timeout
		if timeout == 0 {
			timeout = c.Timeout
		}
		cfgs[name] = openaiClient.NewClientConfig(apiKey, sub.Model, baseURL, timeout)
	}
	return cfgs
}
