// Package client 提供TEMU平台认证管理功能
package client

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

// AuthManager 认证管理器
type AuthManager struct {
	config        *AuthConfig
	retryHandler  RetryHandler
	errorDetector ErrorDetector
	pauseHandler  PauseHandler
	logger        *logrus.Entry
}

// NewAuthManager 创建新的认证管理器
func NewAuthManager(logger *logrus.Entry) *AuthManager {
	config := DefaultAuthConfig()
	errorDetector := NewTemuErrorDetector(config, logger)
	pauseHandler := NewTemuPauseHandler(logger)
	retryHandler := NewTemuRetryHandler(config, errorDetector, pauseHandler, logger)

	return &AuthManager{
		config:        config,
		retryHandler:  retryHandler,
		errorDetector: errorDetector,
		pauseHandler:  pauseHandler,
		logger:        logger,
	}
}

// NewAuthManagerWithDependencies 使用自定义依赖创建认证管理器（用于测试）
func NewAuthManagerWithDependencies(
	config *AuthConfig,
	retryHandler RetryHandler,
	errorDetector ErrorDetector,
	pauseHandler PauseHandler,
	logger *logrus.Entry,
) *AuthManager {
	return &AuthManager{
		config:        config,
		retryHandler:  retryHandler,
		errorDetector: errorDetector,
		pauseHandler:  pauseHandler,
		logger:        logger,
	}
}

// SendRequestWithAuth 发送带认证的请求
func (a *AuthManager) SendRequestWithAuth(client ClientAPI, request map[string]any, result any) error {
	// 检查Cookie
	if err := a.validateCookies(client); err != nil {
		return err
	}

	// 使用重试逻辑发送请求
	return a.retryHandler.SendRequestWithRetry(client, request, result)
}

// validateCookies 验证Cookie状态
func (a *AuthManager) validateCookies(client ClientAPI) error {
	if !client.HasCookies() {
		a.logger.Warnf("店铺ID=%d没有Cookie数据，尝试重新加载Cookie", client.GetStoreID())

		// 尝试重新加载Cookie
		if err := client.ReloadCookies(); err != nil {
			a.logger.Errorf("重新加载Cookie失败: %v", err)

			// 设置暂停键
			if pauseErr := a.pauseHandler.SetPauseKeyForAuthExpired(client, "从管理系统获取Cookie失败: Cookie数据为空"); pauseErr != nil {
				a.logger.Errorf("设置暂停键失败: %v", pauseErr)
			}

			return NewAuthExpiredError(
				fmt.Sprintf("店铺ID=%d没有Cookie数据且重新加载失败，请先在管理系统中设置Cookie", client.GetStoreID()),
				err,
			)
		}

		// 再次检查Cookie
		if !client.HasCookies() {
			a.logger.Warn("Cookie数据为空")

			if pauseErr := a.pauseHandler.SetPauseKeyForAuthExpired(client, "Cookie数据为空"); pauseErr != nil {
				a.logger.Errorf("设置暂停键失败: %v", pauseErr)
			}

			return NewAuthExpiredError(
				fmt.Sprintf("店铺ID=%d没有Cookie数据，请先在管理系统中设置Cookie", client.GetStoreID()),
				nil,
			)
		}

		a.logger.Infof("成功重新加载Cookie，数量: %d", client.GetCookieCount())
	} else {
		a.logger.Debugf("Cookie检查通过，数量: %d", client.GetCookieCount())
	}

	return nil
}
