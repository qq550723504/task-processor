// Package syncsvc 提供TEMU调度服务工厂
package syncsvc

import (
	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
	temuproduct "task-processor/internal/temu/api/product"
	temuquery "task-processor/internal/temu/api/query"
)

// ServiceFactory TEMU调度服务工厂
type ServiceFactory struct {
	managementClient *management.ClientManager
	productAPI       *temuproduct.API
	skuQueryAPI      *temuquery.API
	mappingClient    managementapi.ProductImportMappingAPI
	storeAPI         managementapi.StoreAPI
}

// NewServiceFactory 创建TEMU服务工厂
func NewServiceFactory(
	managementClient *management.ClientManager,
	productAPI *temuproduct.API,
	skuQueryAPI *temuquery.API,
	mappingClient managementapi.ProductImportMappingAPI,
	storeAPI managementapi.StoreAPI,
) *ServiceFactory {
	return &ServiceFactory{
		managementClient: managementClient,
		productAPI:       productAPI,
		skuQueryAPI:      skuQueryAPI,
		mappingClient:    mappingClient,
		storeAPI:         storeAPI,
	}
}

// CreateProductSyncService 创建产品同步服务
func (f *ServiceFactory) CreateProductSyncService() ProductSyncService {
	return NewProductSyncService(
		f.managementClient,
		f.productAPI,
		f.skuQueryAPI,
		f.mappingClient,
		f.storeAPI,
		nil, // 使用默认配置
	)
}

// CreateInventorySyncService 创建库存同步服务
func (f *ServiceFactory) CreateInventorySyncService() InventorySyncService {
	// TODO: 返回具体实现
	return nil
}
