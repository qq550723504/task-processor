package service

import (
	"context"

	"task-processor/internal/app/messaging"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

// MessagingService 消息消费端应用服务抽象
// 目前直接由 messaging.ServiceManager 实现，后续可以在不影响调用方的情况下重构内部结构。
type MessagingService interface {
	// RegisterProcessor 注册平台任务处理器
	RegisterProcessor(platform string, processor worker.Processor) error
	// Start 启动消息消费等相关服务
	Start(ctx context.Context) error
	// Stop 停止消息消费等相关服务
	Stop(ctx context.Context) error
	// GetStats 返回当前服务的运行统计信息
	GetStats() map[string]interface{}
	// IsStarted 返回服务是否已启动
	IsStarted() bool
}

// NewMessagingService 创建基于 RabbitMQ 的消息服务
// 目前返回 messaging.ServiceManager 作为默认实现。
func NewMessagingService(rabbitmqConfig *config.RabbitMQConfig, logger *logrus.Logger) (MessagingService, error) {
	return messaging.NewServiceManager(rabbitmqConfig, logger)
}

