// Package service 提供服务工厂，管理依赖注入
package service

import (
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/openai"
	"task-processor/internal/platforms/amazon/core/model"

	"github.com/sirupsen/logrus"
)

// ServiceFactory 服务工厂
type ServiceFactory struct {
	config    *config.Config
	logger    *logrus.Entry
	llmClient LLMClient
}

// NewServiceFactory 创建服务工厂
func NewServiceFactory(cfg *config.Config) *ServiceFactory {
	return &ServiceFactory{
		config: cfg,
		logger: logrus.WithField("component", "ServiceFactory"),
	}
}

// CreateLLMAttributeMapper 创建LLM属性映射器
func (f *ServiceFactory) CreateLLMAttributeMapper() *LLMAttributeMapper {
	if f.llmClient == nil {
		f.llmClient = f.createLLMClient()
	}

	return NewLLMAttributeMapper(f.llmClient)
}

// createLLMClient 创建LLM客户端
func (f *ServiceFactory) createLLMClient() LLMClient {
	// 创建OpenAI客户端配置
	openaiConfig := openai.NewClientConfig(
		f.config.OpenAI.APIKey,
		f.config.OpenAI.Model,
		f.config.OpenAI.BaseURL,
		f.config.OpenAI.Timeout,
	)

	// 创建OpenAI客户端
	openaiClient := openai.NewClient(openaiConfig)

	// 创建LLM客户端适配器
	llmClient := NewOpenAILLMClient(openaiClient)

	f.logger.WithFields(logrus.Fields{
		"model":    openaiConfig.Model,
		"base_url": openaiConfig.BaseURL,
	}).Info("LLM客户端创建成功")

	return llmClient
}

// UpdateServices 更新服务容器中的服务
func (f *ServiceFactory) UpdateServices(services *model.Services) {
	// 创建LLM属性映射器
	llmMapper := f.CreateLLMAttributeMapper()

	// 将服务注入到容器中
	services.SetLLMAttributeMapper(llmMapper)

	f.logger.Info("服务容器更新完成")
}
