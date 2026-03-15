// Package service 提供处理器管理功能
package service

import (
	"context"
	"fmt"

	"task-processor/internal/core/config"
	"task-processor/internal/platforms/shein/pipeline"
	"task-processor/internal/platforms/temu"
)

// startProcessors 启动所有平台处理器
func (s *processorServiceImpl) startProcessors(ctx context.Context, cfg *config.Config) error {
	s.logger.Info("开始启动处理器...")

	// 检查并启动TEMU处理器
	if cfg.Platforms.Temu.Enabled {
		s.logger.Info("TEMU处理器已启用，正在启动...")
		if err := s.startTemuProcessor(ctx, cfg); err != nil {
			return fmt.Errorf("启动TEMU处理器失败: %w", err)
		}
	} else {
		s.logger.Info("TEMU处理器已禁用，跳过启动")
	}

	// 检查并启动SHEIN处理器
	if cfg.Platforms.Shein.Enabled {
		s.logger.Info("SHEIN处理器已启用，正在启动...")
		if err := s.startSheinProcessor(ctx, cfg); err != nil {
			return fmt.Errorf("启动SHEIN处理器失败: %w", err)
		}
	} else {
		s.logger.Info("SHEIN处理器已禁用，跳过启动")
	}

	// 检查是否至少启用了一个处理器
	if !cfg.Platforms.Temu.Enabled && !cfg.Platforms.Shein.Enabled {
		s.logger.Warn("⚠️ 所有处理器都已禁用，系统将以空闲模式运行")
	}

	s.logger.Info("处理器启动流程完成")
	return nil
}

// startTemuProcessor 启动TEMU处理器
func (s *processorServiceImpl) startTemuProcessor(ctx context.Context, cfg *config.Config) error {
	s.logger.Info("启动TEMU处理器...")

	// 使用注入的Amazon处理器
	if s.amazonProcessor == nil {
		return fmt.Errorf("Amazon处理器未初始化")
	}

	// 创建TEMU处理器，使用注入的资源
	// 注意：这里传递 nil 作为 rabbitmqClient，因为旧的启动方式不使用 RabbitMQ
	// 如果需要使用分布式爬虫，请使用 cmd/rabbitmq-consumer 启动程序
	temuProcessor, err := temu.NewTemuProcessor(ctx, cfg, s.logger, s.managementClient, s.amazonProcessor, nil)
	if err != nil {
		return fmt.Errorf("创建TEMU处理器失败: %w", err)
	}
	s.temuProcessor = temuProcessor

	// 启动处理器
	if err := s.temuProcessor.Start(ctx); err != nil {
		return fmt.Errorf("TEMU处理器启动失败: %w", err)
	}

	s.logger.Info("✅ TEMU处理器启动完成")
	return nil
}

// startSheinProcessor 启动SHEIN处理器
func (s *processorServiceImpl) startSheinProcessor(ctx context.Context, cfg *config.Config) error {
	s.logger.Info("启动SHEIN处理器...")

	// 使用注入的Amazon处理器
	if s.amazonProcessor == nil {
		return fmt.Errorf("Amazon处理器未初始化")
	}

	// 创建SHEIN处理器，使用注入的资源
	// 注意：这里传递 nil 作为 rabbitmqClient，因为旧的启动方式不使用 RabbitMQ
	// 如果需要使用分布式爬虫，请使用 cmd/rabbitmq-consumer 启动程序
	sheinProcessor, err := pipeline.NewSheinProcessor(ctx, cfg, s.logger, s.managementClient, s.amazonProcessor, nil)
	if err != nil {
		return fmt.Errorf("创建SHEIN处理器失败: %w", err)
	}
	s.sheinProcessor = sheinProcessor

	// 启动处理器
	if err := s.sheinProcessor.Start(ctx); err != nil {
		return fmt.Errorf("SHEIN处理器启动失败: %w", err)
	}

	s.logger.Info("✅ SHEIN处理器启动完成")
	return nil
}

// stopAllProcessors 停止所有处理器
func (s *processorServiceImpl) stopAllProcessors(ctx context.Context) {
	// 停止TEMU处理器
	if s.temuProcessor != nil {
		s.temuProcessor.Close(ctx)
		s.logger.Info("TEMU处理器已停止")
	}

	// 停止SHEIN处理器
	if s.sheinProcessor != nil {
		s.sheinProcessor.Close(ctx)
		s.logger.Info("SHEIN处理器已停止")
	}

	// Amazon处理器和管理客户端由依赖注入容器管理生命周期
	// 不需要手动关闭，容器会在应用关闭时自动处理
	s.logger.Info("✅ 所有处理器已停止")
}

