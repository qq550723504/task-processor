// Package listing 提供服务工厂，管理依赖注入
package listing

import (
	"task-processor/internal/amazon/llm"
	"task-processor/internal/amazon/model"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/openai"

	"github.com/sirupsen/logrus"
)

// ServiceFactory 服务工厂
type ServiceFactory struct {
	config    *config.Config
	logger    *logrus.Entry
	llmClient llm.LLMClient
}

// NewServiceFactory 创建服务工厂
func NewServiceFactory(cfg *config.Config) *ServiceFactory {
	return &ServiceFactory{
		config: cfg,
		logger: logrus.WithField("component", "ServiceFactory"),
	}
}

// CreateLLMAttributeMapper 创建LLM属性映射器
func (f *ServiceFactory) CreateLLMAttributeMapper() *llm.LLMAttributeMapper {
	if f.llmClient == nil {
		f.llmClient = f.createLLMClient()
	}
	return llm.NewLLMAttributeMapper(f.llmClient)
}

// createLLMClient 创建LLM客户端
func (f *ServiceFactory) createLLMClient() llm.LLMClient {
	openaiConfig := openai.NewClientConfig(
		f.config.OpenAI.APIKey,
		f.config.OpenAI.Model,
		f.config.OpenAI.BaseURL,
		f.config.OpenAI.Timeout,
	)
	openaiClient := openai.NewClient(openaiConfig)
	llmClient := llm.NewOpenAILLMClient(openaiClient)

	f.logger.WithFields(logrus.Fields{
		"model":    openaiConfig.Model,
		"base_url": openaiConfig.BaseURL,
	}).Info("LLM客户端创建成功")

	return llmClient
}

// UpdateServices 更新服务容器中的服务
func (f *ServiceFactory) UpdateServices(services *model.Services) {
	llmMapper := f.CreateLLMAttributeMapper()
	services.SetLLMAttributeMapper(llmMapper)
	f.logger.Info("服务容器更新完成")
}
