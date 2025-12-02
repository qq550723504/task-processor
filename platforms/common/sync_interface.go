package common

import (
	"task-processor/common/management/api"
)

// ProductSyncService 产品同步服务接口
// 所有平台的同步服务都需要实现此接口
type ProductSyncService interface {
	// SyncProducts 同步产品列表
	// storeID: 店铺 ID
	// 返回同步成功的产品数量和错误
	SyncProducts(storeID int64) (int, error)

	// SyncSingleProduct 同步单个产品
	// storeID: 店铺 ID
	// platformProductID: 平台商品 ID
	SyncSingleProduct(storeID int64, platformProductID string) error

	// MapToProductData 将平台原始数据映射为通用产品数据
	// rawData: 平台原始数据
	MapToProductData(rawData interface{}) (*api.ProductDataDTO, error)

	// MapShelfStatus 映射平台状态到统一上架状态
	// platformStatus: 平台原始状态
	MapShelfStatus(platformStatus interface{}) int

	// GetPlatformName 获取平台名称
	GetPlatformName() string
}
