// Package service 提供处理器生命周期管理
package service

import (
	"context"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/core/errors"
	"task-processor/internal/infra/auth"
)

// StartProcessors 启动所有处理器
func (s *processorServiceImpl) StartProcessors(ctx context.Context, cfg *config.Config, authClient *auth.ClientCredentialsAuthClient) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return errors.New(errors.ErrCodeSystem, "处理器已经在运行中")
	}

	s.logger.Info("🚀 开始启动任务处理器...")

	// 创建上下文
	s.ctx, s.cancel = context.WithCancel(ctx)

	// 验证依赖注入的资源
	if s.managementClient == nil {
		return errors.New(errors.ErrCodeSystem, "管理客户端未注入")
	}
	if s.amazonProcessor == nil {
		return errors.New(errors.ErrCodeSystem, "Amazon处理器未注入")
	}

	// 启动处理器
	if err := s.startProcessors(ctx, cfg); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "启动处理器失败")
	}

	// 启动任务获取器
	if err := s.startTaskFetcher(cfg); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "启动任务获取器失败")
	}

	// 启动调度服务
	if err := s.startSchedulerService(ctx, cfg); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "启动调度服务失败")
	}

	// 初始化监控组件
	if err := s.initializeMonitoring(cfg); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "初始化监控组件失败")
	}

	// 启动所有组件
	if err := s.lifecycleManager.StartAll(s.ctx); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "启动组件失败")
	}

	s.running = true
	s.logger.Info("✅ 所有任务处理器启动完成")

	// 启动状态监控
	go s.startStatusMonitor()

	return nil
}

// StopProcessors 停止所有处理器
func (s *processorServiceImpl) StopProcessors() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		s.logger.Info("处理器服务未运行，无需停止")
		return nil
	}

	s.logger.Info("🛑 开始停止任务处理器...")

	// 取消上下文
	if s.cancel != nil {
		s.cancel()
	}

	// 使用生命周期管理器停止所有组件
	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var lastError error

	// 停止生命周期管理器管理的组件
	if err := s.lifecycleManager.StopAll(stopCtx); err != nil {
		s.logger.Errorf("停止生命周期管理器组件失败: %v", err)
		lastError = err
	}

	// 停止其他处理器
	s.stopAllProcessors(stopCtx)

	// 停止调度服务
	if s.schedulerService != nil {
		if err := s.schedulerService.Stop(stopCtx); err != nil {
			s.logger.Errorf("停止调度服务失败: %v", err)
			lastError = err
		}
	}

	s.running = false

	if lastError != nil {
		s.logger.Warn("⚠️ 部分组件停止时发生错误，但主要组件已停止")
		return errors.Wrap(lastError, errors.ErrCodeSystem, "部分组件停止失败")
	}

	s.logger.Info("✅ 所有任务处理器已停止")
	return nil
}

// GetStatus 获取处理器状态
func (s *processorServiceImpl) GetStatus() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := map[string]any{
		"running": s.running,
		"processors": map[string]any{
			"temu":  s.temuProcessor != nil,
			"shein": s.sheinProcessor != nil,
		},
		"taskFetcher": s.taskFetcher != nil && s.taskFetcher.IsRunning(),
		"components":  s.lifecycleManager.GetStatus(),
	}

	// 添加调度服务状态
	if s.schedulerService != nil {
		status["schedulerService"] = s.schedulerService.GetStatus()
	}

	// 添加监控组件状态
	if s.metricsCollector != nil {
		status["metricsCollector"] = s.metricsCollector.IsRunning()
		status["metrics"] = s.metricsCollector.GetMetrics()
	}

	if s.healthChecker != nil {
		status["healthChecker"] = s.healthChecker.IsRunning()
	}

	return status
}
