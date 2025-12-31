// Package browser 提供浏览器指纹生成功能
package browser

import (
	"fmt"
	"time"
)

// FingerprintGenerator 指纹生成器（简化版）
type FingerprintGenerator struct{}

// NewFingerprintGenerator 创建指纹生成器
func NewFingerprintGenerator() *FingerprintGenerator {
	return &FingerprintGenerator{}
}

// GenerateFingerprint 生成指纹
func (fg *FingerprintGenerator) GenerateFingerprint(userID string) *FingerprintConfig {
	return &FingerprintConfig{
		Enable: true,
		GPU: map[string]interface{}{
			"description": "NVIDIA GeForce GTX 1060",
		},
	}
}

// GenerateRandomFingerprint 生成随机指纹
func (fg *FingerprintGenerator) GenerateRandomFingerprint() *FingerprintConfig {
	userID := fmt.Sprintf("random_%d", time.Now().UnixNano())
	return fg.GenerateFingerprint(userID)
}

// GenerateUniqueFingerprint 为实例生成唯一指纹
func (fg *FingerprintGenerator) GenerateUniqueFingerprint(instanceID int) *FingerprintConfig {
	userID := fmt.Sprintf("instance_%d_%d", instanceID, time.Now().UnixNano())
	return fg.GenerateFingerprint(userID)
}

// ValidateFingerprint 验证指纹配置
func (fg *FingerprintGenerator) ValidateFingerprint(config *FingerprintConfig) bool {
	if config == nil {
		return false
	}

	if !config.Enable {
		return true // 禁用状态也是有效的
	}

	// 检查GPU配置
	if config.GPU == nil {
		return false
	}

	if _, exists := config.GPU["description"]; !exists {
		return false
	}

	return true
}

// GetDefaultFingerprint 获取默认指纹配置
func (fg *FingerprintGenerator) GetDefaultFingerprint() *FingerprintConfig {
	return &FingerprintConfig{
		Enable: false, // 默认禁用
		GPU:    make(map[string]interface{}),
	}
}
