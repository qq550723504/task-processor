// Package service 提供处理器服务实现
package service

import (
	"context"
	"fmt"

	"task-processor/internal/auth"
	"task-processor/internal/common/management"
	"task-processor/internal/common/task"
	"task-processor/internal/config"
	"task-processor/internal/platforms/shein"
	"task-processor/internal/platforms/temu"

	"github.com/sirupsen/logrus"
)

// processorServiceImpl 处理器服务实现
type processorServiceImpl struct {
	logger           *logrus.Logger
	temuProcessor    *temu.TemuProcessor
	sheinProcessor   *shein.SheinProcessor
	taskFetcher      *task.UnifiedTaskFetcher
	managementClient *management.ClientManager
	ctx              context.Context
	cancel           context.CancelFunc
	running          bool
}

// StartProcessors 启动所有处理器
func (s *processorServiceImpl) StartProcessors(ctx context.Context, cfg *config.Config, authClient *auth.ClientCredentialsAuthClient) error {
	if s.running {
		return fmt.Errorf("处理器已经在运行中")
	}

	s.logger.Info("🚀 开始启动任务处理器...")

	// 创建上下文
	s.ctx, s.cancel = context.WithCancel(ctx)

	// 初始化管理客户端
	if err := s.initializeManagementClient(cfg, authClient); err != nil {
		return fmt.Errorf("初始化管理客户端失败: %w", err)
	}

	// 启动处理器
	if err := s.startProcessors(cfg); err != nil {
		return fmt.Errorf("启动处理器失败: %w", err)
	}

	// 启动任务获取器
	if err := s.startTaskFetcher(cfg); err != nil {
		return fmt.Errorf("启动任务获取器失败: %w", err)
	}

	s.running = true
	s.logger.Info("✅ 所有任务处理器启动完成")

	// 启动状态监控
	go s.startStatusMonitor()

	return nil
}

// StopProcessors 停止所有处理器
func (s *processorServiceImpl) StopProcessors() error {
	if !s.running {
		return nil
	}

	s.logger.Info("🛑 开始停止任务处理器...")

	// 取消上下文
	if s.cancel != nil {
		s.cancel()
	}

	// 停止处理器
	s.stopAllProcessors()

	s.running = false
	s.logger.Info("✅ 所有任务处理器已停止")

	return nil
}

// GetStatus 获取处理器状态
func (s *processorServiceImpl) GetStatus() map[string]any {
	return map[string]any{
		"running": s.running,
		"processors": map[string]any{
			"temu":  s.temuProcessor != nil,
			"shein": s.sheinProcessor != nil,
		},
		"taskFetcher": s.taskFetcher != nil,
	}
}
