// Package client 提供TEMU平台认证组件工厂
package client

import (
	"github.com/sirupsen/logrus"
)

// AuthManagerFactory 认证管理器工厂
type AuthManagerFactory struct {
	logger *logrus.Entry
}

// NewAuthManagerFactory 创建认证管理器工厂
func NewAuthManagerFactory(logger *logrus.Entry) *AuthManagerFactory {
	return &AuthManagerFactory{
		logger: logger,
	}
}

// CreateAuthManager 创建标准认证管理器
func (f *AuthManagerFactory) CreateAuthManager() *AuthManager {
	return NewAuthManager(f.logger)
}

// CreateAuthManagerWithCustomConfig 使用自定义配置创建认证管理器
func (f *AuthManagerFactory) CreateAuthManagerWithCustomConfig(config *AuthConfig) *AuthManager {
	errorDetector := NewTemuErrorDetector(config, f.logger)
	pauseHandler := NewTemuPauseHandler(f.logger)
	retryHandler := NewTemuRetryHandler(config, errorDetector, pauseHandler, f.logger)

	return NewAuthManagerWithDependencies(config, retryHandler, errorDetector, pauseHandler, f.logger)
}

// CreateComponents 创建所有认证组件（用于测试）
func (f *AuthManagerFactory) CreateComponents(config *AuthConfig) (
	ErrorDetector,
	PauseHandler,
	RetryHandler,
) {
	errorDetector := NewTemuErrorDetector(config, f.logger)
	pauseHandler := NewTemuPauseHandler(f.logger)
	retryHandler := NewTemuRetryHandler(config, errorDetector, pauseHandler, f.logger)

	return errorDetector, pauseHandler, retryHandler
}
