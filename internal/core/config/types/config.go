// Package types 提供配置类型定义
package types

// Config 主配置结构体
type Config struct {
	Processor  ProcessorConfig  `yaml:"processor"`
	Worker     WorkerConfig     `yaml:"worker"`
	OpenAI     OpenAIConfig     `yaml:"openai"`
	Management ManagementConfig `yaml:"management"`
	Browser    BrowserConfig    `yaml:"browser"`
	Amazon     AmazonConfig     `yaml:"amazon"`
	RabbitMQ   *RabbitMQConfig  `yaml:"rabbitmq"`
	Updater    UpdaterConfig    `yaml:"updater"`
	Platforms  PlatformsConfig  `yaml:"platforms"`
}
