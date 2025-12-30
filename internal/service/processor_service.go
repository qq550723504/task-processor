// Package service 提供处理器服务层功能
package service

import (
	"context"

	"task-processor/internal/auth"
	"task-processor/internal/config"
	"task-processor/internal/lifecycle"

	"github.com/sirupsen/logrus"
)

// ProcessorService 处理器服务接口
type ProcessorService interface {
	// StartProcessors 启动所有处理器
	StartProcessors(ctx context.Context, cfg *config.Config, authClient *auth.ClientCredentialsAuthClient) error
	// StopProcessors 停止所有处理器
	StopProcessors() error
	// GetStatus 获取处理器状态
	GetStatus() map[string]any
}

// NewProcessorService 创建处理器服务
func NewProcessorService(logger *logrus.Logger) ProcessorService {
	return &processorServiceImpl{
		logger:           logger,
		lifecycleManager: lifecycle.NewManager(logger),
	}
}
