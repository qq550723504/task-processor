// Package client 提供TEMU平台认证错误检测功能
package client

import (
	"strings"

	"github.com/sirupsen/logrus"
)

// ErrorDetector 认证错误检测器接口
type ErrorDetector interface {
	IsAuthenticationError(err error) bool
}

// TemuErrorDetector TEMU平台认证错误检测器
type TemuErrorDetector struct {
	config *AuthConfig
	logger *logrus.Entry
}

// NewTemuErrorDetector 创建新的TEMU错误检测器
func NewTemuErrorDetector(config *AuthConfig, logger *logrus.Entry) *TemuErrorDetector {
	return &TemuErrorDetector{
		config: config,
		logger: logger,
	}
}

// IsAuthenticationError 判断是否为认证相关错误
func (d *TemuErrorDetector) IsAuthenticationError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// 检查TEMU特定的认证错误码
	if d.containsTemuAuthErrorCode(errStr) {
		return true
	}

	// 检查通用认证错误关键词
	if d.containsAuthErrorKeyword(errStr) {
		return true
	}

	return false
}

// containsTemuAuthErrorCode 检查是否包含TEMU认证错误码
func (d *TemuErrorDetector) containsTemuAuthErrorCode(errStr string) bool {
	for _, errorCode := range d.config.TemuAuthErrorCodes {
		if strings.Contains(errStr, errorCode) {
			d.logger.Debugf("检测到TEMU认证错误码: %s", errorCode)
			return true
		}
	}
	return false
}

// containsAuthErrorKeyword 检查是否包含认证错误关键词
func (d *TemuErrorDetector) containsAuthErrorKeyword(errStr string) bool {
	for _, keyword := range d.config.AuthErrorKeywords {
		if strings.Contains(errStr, keyword) {
			d.logger.Debugf("检测到认证错误关键词: %s", keyword)
			return true
		}
	}
	return false
}
