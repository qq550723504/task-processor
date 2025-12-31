// Package service 提供全局资源访问功能
package service

import (
	"task-processor/internal/common/management"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/memory"
)

var (
	// 全局资源引用
	globalConfig        *config.Config
	globalMemoryManager *memory.MemoryManager
)

// SetGlobalConfig 设置全局配置
func SetGlobalConfig(cfg *config.Config) {
	globalConfig = cfg
}

// GetGlobalConfig 获取全局配置
func GetGlobalConfig() *config.Config {
	return globalConfig
}

// SetGlobalMemoryManager 设置全局内存管理器
func SetGlobalMemoryManager(memoryManager *memory.MemoryManager) {
	globalMemoryManager = memoryManager
}

// GetSharedMemoryManager 获取共享内存管理器
func GetSharedMemoryManager() *memory.MemoryManager {
	return globalMemoryManager
}

// GetSharedManagementClientInstance 获取共享管理客户端实例
func GetSharedManagementClientInstance() *management.ClientManager {
	managementClientMutex.Lock()
	defer managementClientMutex.Unlock()
	return sharedManagementClient
}
