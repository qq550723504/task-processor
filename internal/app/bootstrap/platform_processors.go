// Package bootstrap 提供平台处理器构造函数
package bootstrap

import (
	"context"
	"fmt"

	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"

	"github.com/sirupsen/logrus"
)

// buildTemuProcessor 构造 TEMU 处理器
func buildTemuProcessor(svc *appServices, logger *logrus.Logger) (*temu.TemuProcessor, error) {
	// 注意：传递 nil 作为 rabbitmqClient，旧启动方式不使用 RabbitMQ
	// 如需分布式爬虫，请使用 cmd/rabbitmq-consumer
	proc, err := temu.NewTemuProcessor(
		context.Background(),
		svc.cfg,
		logger,
		svc.managementClient,
		svc.amazonCrawler,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("创建TEMU处理器失败: %w", err)
	}
	return proc, nil
}

// buildSheinProcessor 构造 SHEIN 处理器
func buildSheinProcessor(svc *appServices, logger *logrus.Logger) (*pipeline.SheinProcessor, error) {
	// 注意：传递 nil 作为 rabbitmqClient，旧启动方式不使用 RabbitMQ
	// 如需分布式爬虫，请使用 cmd/rabbitmq-consumer
	proc, err := pipeline.NewSheinProcessor(
		context.Background(),
		svc.cfg,
		logger,
		svc.managementClient,
		svc.amazonCrawler,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("创建SHEIN处理器失败: %w", err)
	}
	return proc, nil
}
