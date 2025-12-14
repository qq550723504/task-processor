// Package service 提供业务逻辑层
package service

import (
	"fmt"

	"task-processor/common/auth"
	"task-processor/internal/config"

	"github.com/sirupsen/logrus"
)

// AuthService 认证服务
type AuthService struct {
	logger *logrus.Logger
}

// NewAuthService 创建认证服务实例
func NewAuthService(logger *logrus.Logger) *AuthService {
	return &AuthService{
		logger: logger,
	}
}

// InitializeClientCredentials 初始化客户端凭证认证
func (s *AuthService) InitializeClientCredentials(cfg *config.Config) (*auth.ClientCredentialsAuthClient, error) {
	s.logger.Info("初始化客户端凭证授权...")

	// 从配置中获取租户ID
	tenantID := cfg.Management.TenantID
	if tenantID == "" {
		tenantID = "1"
	}

	client := auth.NewClientCredentialsAuthClient(
		cfg.Management.BaseURL,
		cfg.Management.ClientID,
		cfg.Management.ClientSecret,
		tenantID,
	)

	// 立即获取一次token，验证配置是否正确
	_, err := client.GetAccessToken()
	if err != nil {
		return nil, fmt.Errorf("获取访问令牌失败: %w", err)
	}

	return client, nil
}
