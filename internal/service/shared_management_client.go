// Package service 提供共享管理客户端功能
package service

import (
	"sync"

	"task-processor/internal/common/management"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/auth"

	"github.com/sirupsen/logrus"
)

var (
	// 全局管理客户端单例
	sharedManagementClient *management.ClientManager
	managementClientMutex  sync.Mutex
)

// GetSharedManagementClient 获取共享的管理客户端
func GetSharedManagementClient(cfg *config.Config, authClient *auth.ClientCredentialsAuthClient, logger *logrus.Logger) (*management.ClientManager, error) {
	managementClientMutex.Lock()
	defer managementClientMutex.Unlock()

	if sharedManagementClient == nil {
		logger.Info("🔄 创建共享管理客户端...")

		// 获取访问令牌
		accessToken, err := authClient.GetAccessToken()
		if err != nil {
			return nil, err
		}

		// 创建管理客户端
		sharedManagementClient = management.NewClientManager(&cfg.Management)

		// 设置访问令牌到客户端实现
		client := sharedManagementClient.GetClient()
		client.SetUserToken(accessToken, cfg.Management.TenantID)

		logger.Info("✅ 共享管理客户端创建完成")
	} else {
		logger.Info("♻️ 复用现有管理客户端")
	}

	return sharedManagementClient, nil
}

// CloseSharedManagementClient 关闭共享的管理客户端
func CloseSharedManagementClient(logger *logrus.Logger) {
	managementClientMutex.Lock()
	defer managementClientMutex.Unlock()

	if sharedManagementClient != nil {
		logger.Info("🛑 关闭共享管理客户端...")
		// 注意：management.ClientManager 可能没有Close方法，
		// 这里只是清理引用，让GC回收
		sharedManagementClient = nil
		logger.Info("✅ 共享管理客户端已关闭")
	}
}
