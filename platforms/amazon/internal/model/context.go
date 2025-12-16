// Package model 提供Amazon平台内部数据模型
package model

import (
	"task-processor/common/management"
	"task-processor/common/memory"
	"task-processor/platforms/amazon/api"
)

// Services Amazon平台服务集合
// 用于依赖注入，避免循环导入
type Services struct {
	APIClient        *api.Client
	ProductTypeCache any // 使用any避免循环导入
	ManagementClient *management.ClientManager
	MemoryManager    *memory.MemoryManager
}

// NewServices 创建服务集合
func NewServices() *Services {
	return &Services{}
}

// SetAPIClient 设置API客户端
func (s *Services) SetAPIClient(client *api.Client) {
	s.APIClient = client
}

// SetProductTypeCache 设置产品类型缓存
func (s *Services) SetProductTypeCache(cache any) {
	s.ProductTypeCache = cache
}

// GetProductTypeCache 获取产品类型缓存
func (s *Services) GetProductTypeCache() any {
	return s.ProductTypeCache
}

// SetManagementClient 设置管理客户端
func (s *Services) SetManagementClient(client *management.ClientManager) {
	s.ManagementClient = client
}

// SetMemoryManager 设置内存管理器
func (s *Services) SetMemoryManager(manager *memory.MemoryManager) {
	s.MemoryManager = manager
}
