// Package service 提供共享资源管理功能
package service

import (
	"fmt"

	"task-processor/internal/auth"
	"task-processor/internal/config"
)

// initializeSharedResources 初始化所有共享资源
func (s *processorServiceImpl) initializeSharedResources(cfg *config.Config, authClient *auth.ClientCredentialsAuthClient) error {
	s.logger.Info("🔄 开始初始化共享资源...")

	// 1. 初始化共享管理客户端
	if err := s.initializeSharedManagementClient(cfg, authClient); err != nil {
		return fmt.Errorf("初始化共享管理客户端失败: %w", err)
	}

	// 2. 初始化共享Amazon处理器
	if err := s.initializeSharedAmazonProcessor(cfg); err != nil {
		return fmt.Errorf("初始化共享Amazon处理器失败: %w", err)
	}

	s.logger.Info("✅ 所有共享资源初始化完成")
	return nil
}

// initializeSharedManagementClient 初始化共享管理客户端
func (s *processorServiceImpl) initializeSharedManagementClient(cfg *config.Config, authClient *auth.ClientCredentialsAuthClient) error {
	s.logger.Info("初始化共享管理客户端...")

	// 使用共享的管理客户端
	managementClient, err := GetSharedManagementClient(cfg, authClient, s.logger)
	if err != nil {
		return fmt.Errorf("获取共享管理客户端失败: %w", err)
	}

	s.managementClient = managementClient
	s.logger.Info("✅ 共享管理客户端初始化完成")
	return nil
}

// initializeSharedAmazonProcessor 初始化共享Amazon处理器
func (s *processorServiceImpl) initializeSharedAmazonProcessor(cfg *config.Config) error {
	s.logger.Info("初始化共享Amazon处理器...")

	// 确保Amazon配置已启用
	if !cfg.Amazon.Enabled {
		s.logger.Info("Amazon处理器未启用，跳过初始化")
		return nil
	}

	// 获取共享的Amazon处理器（这会创建实例如果不存在）
	_ = GetSharedAmazonProcessor(cfg, s.logger)

	s.logger.Info("✅ 共享Amazon处理器初始化完成")
	return nil
}
