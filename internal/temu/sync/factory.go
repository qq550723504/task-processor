// package sync 提供TEMU调度服务工厂
package sync

import (
	"task-processor/internal/listingadmin"
	temuproduct "task-processor/internal/temu/api/product"
	temuquery "task-processor/internal/temu/api/query"
)

// ServiceFactory TEMU调度服务工厂
type ServiceFactory struct {
	runtime       productSyncRuntime
	productAPI    *temuproduct.API
	skuQueryAPI   *temuquery.API
	mappingClient listingadmin.ProductImportMappingAPI
	storeAPI      listingadmin.StoreAPI
}

// NewServiceFactory 创建TEMU服务工厂
func NewServiceFactory(
	runtime productSyncRuntime,
	productAPI *temuproduct.API,
	skuQueryAPI *temuquery.API,
	mappingClient listingadmin.ProductImportMappingAPI,
	storeAPI listingadmin.StoreAPI,
) *ServiceFactory {
	return &ServiceFactory{
		runtime:       runtime,
		productAPI:    productAPI,
		skuQueryAPI:   skuQueryAPI,
		mappingClient: mappingClient,
		storeAPI:      storeAPI,
	}
}

// CreateProductSyncService 创建产品同步服务
func (f *ServiceFactory) CreateProductSyncService() ProductSyncService {
	return NewProductSyncService(
		f.runtime,
		f.productAPI,
		f.skuQueryAPI,
		f.mappingClient,
		f.storeAPI,
		nil, // 使用默认配置
	)
}
