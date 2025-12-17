// Package service 提供处理器管理功能
package service

import (
	"fmt"

	"task-processor/internal/config"
	"task-processor/internal/platforms/shein"
	"task-processor/internal/platforms/temu"
)

// startProcessors 启动所有平台处理器
func (s *processorServiceImpl) startProcessors(cfg *config.Config) error {
	// 启动TEMU处理器
	if err := s.startTemuProcessor(cfg); err != nil {
		return fmt.Errorf("启动TEMU处理器失败: %w", err)
	}

	// 启动SHEIN处理器
	if err := s.startSheinProcessor(cfg); err != nil {
		return fmt.Errorf("启动SHEIN处理器失败: %w", err)
	}

	return nil
}

// startTemuProcessor 启动TEMU处理器
func (s *processorServiceImpl) startTemuProcessor(cfg *config.Config) error {
	s.logger.Info("启动TEMU处理器...")

	// 获取共享的Amazon处理器
	sharedAmazonProcessor := GetSharedAmazonProcessor(cfg, s.logger)

	// 创建TEMU处理器，使用共享Amazon处理器
	s.temuProcessor = temu.NewTemuProcessorWithSharedAmazon(cfg, s.logger, s.managementClient, sharedAmazonProcessor)

	// 设置用户令牌
	client := s.managementClient.GetClient()
	accessToken, _ := client.GetAccessToken()
	s.temuProcessor.SetUserToken(accessToken, cfg.Management.TenantID)

	// 启动处理器
	if err := s.temuProcessor.Start(s.ctx); err != nil {
		return fmt.Errorf("TEMU处理器启动失败: %w", err)
	}

	s.logger.Info("✅ TEMU处理器启动完成")
	return nil
}

// startSheinProcessor 启动SHEIN处理器
func (s *processorServiceImpl) startSheinProcessor(cfg *config.Config) error {
	s.logger.Info("启动SHEIN处理器...")

	// 获取共享的Amazon处理器
	sharedAmazonProcessor := GetSharedAmazonProcessor(cfg, s.logger)

	// 创建SHEIN处理器，使用共享Amazon处理器
	s.sheinProcessor = shein.NewSheinProcessorWithSharedResources(cfg, s.managementClient, sharedAmazonProcessor)

	// 设置用户令牌
	client := s.managementClient.GetClient()
	accessToken, _ := client.GetAccessToken()
	s.sheinProcessor.SetUserToken(accessToken, cfg.Management.TenantID)

	// 启动处理器
	if err := s.sheinProcessor.Start(s.ctx); err != nil {
		return fmt.Errorf("SHEIN处理器启动失败: %w", err)
	}

	s.logger.Info("✅ SHEIN处理器启动完成")
	return nil
}

// stopAllProcessors 停止所有处理器
func (s *processorServiceImpl) stopAllProcessors() {
	// 停止TEMU处理器
	if s.temuProcessor != nil {
		s.temuProcessor.Close()
		s.logger.Info("TEMU处理器已停止")
	}

	// 停止SHEIN处理器
	if s.sheinProcessor != nil {
		s.sheinProcessor.Close()
		s.logger.Info("SHEIN处理器已停止")
	}

	// 关闭共享的Amazon处理器
	CloseSharedAmazonProcessor(s.logger)
}
