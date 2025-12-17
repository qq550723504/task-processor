// Package service 提供管理客户端管理功能
package service

import (
	"fmt"

	"task-processor/internal/auth"
	"task-processor/internal/common/management"
	"task-processor/internal/config"
)

// initializeManagementClient 初始化管理客户端
func (s *processorServiceImpl) initializeManagementClient(cfg *config.Config, authClient *auth.ClientCredentialsAuthClient) error {
	s.logger.Info("初始化管理系统客户端...")

	// 获取访问令牌
	accessToken, err := authClient.GetAccessToken()
	if err != nil {
		return fmt.Errorf("获取访问令牌失败: %w", err)
	}

	// 创建管理客户端
	s.managementClient = management.NewClientManager(&cfg.Management)

	// 设置访问令牌到客户端实现
	client := s.managementClient.GetClient()
	client.SetUserToken(accessToken, cfg.Management.TenantID)

	s.logger.Info("✅ 管理系统客户端初始化完成")
	return nil
}
