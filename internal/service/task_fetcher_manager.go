// Package service 提供任务获取器管理功能
package service

import (
	"fmt"

	"task-processor/internal/config"
	"task-processor/internal/task"
)

// startTaskFetcher 启动任务获取器
func (s *processorServiceImpl) startTaskFetcher(cfg *config.Config) error {
	s.logger.Info("启动统一任务获取器...")

	// 验证配置
	if err := s.validateTaskFetcherConfig(cfg); err != nil {
		return err
	}

	// 创建任务提交器
	submitters := s.createTaskSubmitters()
	if len(submitters) == 0 {
		s.logger.Warn("⚠️ 没有可用的任务提交器，任务获取器暂时跳过启动")
		return nil
	}

	// 创建并启动任务获取器
	s.taskFetcher = task.NewUnifiedTaskFetcher(cfg, s.managementClient, submitters)

	// 设置任务完成通知器
	s.setupTaskCompletionNotifiers()

	go s.taskFetcher.Start(s.ctx)

	s.logger.Info("✅ 统一任务获取器启动完成")
	return nil
}

// setupTaskCompletionNotifiers 设置任务完成通知器
func (s *processorServiceImpl) setupTaskCompletionNotifiers() {
	// 为TEMU处理器设置通知器
	if s.temuProcessor != nil {
		workerPool := s.temuProcessor.GetWorkerPool()
		if workerPool != nil {
			workerPool.SetCompletionNotifier(s.taskFetcher)
			s.logger.Info("✅ TEMU处理器已设置任务完成通知器")
		}
	}

	// 为SHEIN处理器设置通知器
	if s.sheinProcessor != nil {
		workerPool := s.sheinProcessor.GetWorkerPool()
		if workerPool != nil {
			workerPool.SetCompletionNotifier(s.taskFetcher)
			s.logger.Info("✅ SHEIN处理器已设置任务完成通知器")
		}
	}
}

// validateTaskFetcherConfig 验证任务获取器配置
func (s *processorServiceImpl) validateTaskFetcherConfig(cfg *config.Config) error {
	s.logger.Infof("🔍 管理配置检查: UserID=%d, StoreIDs=%v", cfg.Management.UserID, cfg.Management.StoreIDs)

	if cfg.Management.UserID == 0 {
		s.logger.Info("📝 UserID未设置，将获取所有用户的任务")
	}

	if len(cfg.Management.StoreIDs) == 0 {
		s.logger.Warn("⚠️ 管理配置中StoreIDs为空，任务获取器暂时跳过启动，处理器已就绪等待任务")
		return fmt.Errorf("StoreIDs配置为空")
	}

	return nil
}

// createTaskSubmitters 创建任务提交器映射
func (s *processorServiceImpl) createTaskSubmitters() map[string]task.TaskSubmitter {
	submitters := make(map[string]task.TaskSubmitter)

	// 添加TEMU提交器 - 使用WorkerPool实现
	if s.temuProcessor != nil {
		workerPool := s.temuProcessor.GetWorkerPool()
		if workerPool != nil {
			submitters["temu"] = s.temuProcessor.CreateTaskSubmitter(workerPool)
			s.logger.Info("✅ TEMU任务提交器已注册（使用WorkerPool）")
		} else {
			s.logger.Error("❌ TEMU处理器的WorkerPool为空，无法创建任务提交器")
		}
	}

	// 添加SHEIN提交器 - 使用WorkerPool实现
	if s.sheinProcessor != nil {
		workerPool := s.sheinProcessor.GetWorkerPool()
		if workerPool != nil {
			submitters["shein"] = s.sheinProcessor.CreateTaskSubmitter(workerPool)
			s.logger.Info("✅ SHEIN任务提交器已注册（使用WorkerPool）")
		} else {
			s.logger.Error("❌ SHEIN处理器的WorkerPool为空，无法创建任务提交器")
		}
	}

	return submitters
}
