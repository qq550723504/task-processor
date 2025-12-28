// Package service 提供处理器服务实现
package service

import (
	"context"
	"fmt"
	"time"

	"task-processor/internal/auth"
	"task-processor/internal/common/management"
	"task-processor/internal/config"
	"task-processor/internal/platforms/shein"
	"task-processor/internal/platforms/temu"
	"task-processor/internal/task"

	"github.com/sirupsen/logrus"
)

// processorServiceImpl 处理器服务实现
type processorServiceImpl struct {
	logger           *logrus.Logger
	temuProcessor    *temu.TemuProcessor
	sheinProcessor   *shein.SheinProcessor
	taskFetcher      *task.TaskFetcher
	managementClient *management.ClientManager
	pricingService   PricingService // 新增：核价服务
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

	// 初始化共享资源
	if err := s.initializeSharedResources(cfg, authClient); err != nil {
		return fmt.Errorf("初始化共享资源失败: %w", err)
	}

	// 启动处理器
	if err := s.startProcessors(ctx, cfg); err != nil {
		return fmt.Errorf("启动处理器失败: %w", err)
	}

	// 启动任务获取器
	if err := s.startTaskFetcher(cfg); err != nil {
		return fmt.Errorf("启动任务获取器失败: %w", err)
	}

	// 启动核价服务
	if err := s.startPricingService(ctx, cfg); err != nil {
		return fmt.Errorf("启动核价服务失败: %w", err)
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
	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s.stopAllProcessors(stopCtx)

	// 停止核价服务
	if s.pricingService != nil {
		if err := s.pricingService.Stop(stopCtx); err != nil {
			s.logger.Errorf("停止核价服务失败: %v", err)
		}
	}

	s.running = false
	s.logger.Info("✅ 所有任务处理器已停止")

	return nil
}

// GetStatus 获取处理器状态
func (s *processorServiceImpl) GetStatus() map[string]any {
	status := map[string]any{
		"running": s.running,
		"processors": map[string]any{
			"temu":  s.temuProcessor != nil,
			"shein": s.sheinProcessor != nil,
		},
		"taskFetcher": s.taskFetcher != nil,
	}

	// 添加核价服务状态
	if s.pricingService != nil {
		status["pricingService"] = s.pricingService.GetStatus()
	}

	return status
}

// startPricingService 启动核价服务
func (s *processorServiceImpl) startPricingService(ctx context.Context, cfg *config.Config) error {
	s.logger.Info("启动核价服务...")

	// 设置全局资源
	SetGlobalConfig(cfg)

	// 创建核价服务
	s.pricingService = NewPricingService(s.logger)

	// 启动核价服务
	if err := s.pricingService.Start(ctx); err != nil {
		return fmt.Errorf("核价服务启动失败: %w", err)
	}

	s.logger.Info("✅ 核价服务启动完成")
	return nil
}
