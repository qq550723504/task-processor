// Package model 提供Amazon平台内部数据模型
package model

import (
	"task-processor/internal/amazon/api"
	managementapi "task-processor/internal/listingadmin"
	"task-processor/internal/state"
)

// Services Amazon平台服务集合
// 用于依赖注入，避免循环导入
type Services struct {
	APIClient          *api.Client
	ProductTypeCache   any // 使用any避免循环导入
	ProductTypeService any // 产品类型推荐服务
	StoreAPI           managementapi.StoreAPI
	MemoryManager      *state.MemoryManager
	LLMAttributeMapper any // LLM属性映射器
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

// SetStoreAPI 设置店铺 API 端口
func (s *Services) SetStoreAPI(storeAPI managementapi.StoreAPI) {
	s.StoreAPI = storeAPI
}

// SetMemoryManager 设置内存管理器
func (s *Services) SetMemoryManager(manager *state.MemoryManager) {
	s.MemoryManager = manager
}

// SetProductTypeService 设置产品类型服务
func (s *Services) SetProductTypeService(service any) {
	s.ProductTypeService = service
}

// GetProductTypeService 获取产品类型服务
func (s *Services) GetProductTypeService() any {
	return s.ProductTypeService
}

// SetLLMAttributeMapper 设置LLM属性映射器
func (s *Services) SetLLMAttributeMapper(mapper any) {
	s.LLMAttributeMapper = mapper
}

// GetLLMAttributeMapper 获取LLM属性映射器
func (s *Services) GetLLMAttributeMapper() any {
	return s.LLMAttributeMapper
}
