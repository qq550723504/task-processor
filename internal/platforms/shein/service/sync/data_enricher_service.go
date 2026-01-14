// Package sync 提供SHEIN平台产品数据增强功能
package sync

import (
	"task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/api/product"
	"task-processor/internal/platforms/shein/model"

	"github.com/sirupsen/logrus"
)

// DataEnricher 数据增强器
type DataEnricher struct {
	mappingClient api.ProductImportMappingAPI
	logger        *logrus.Entry
}

// NewDataEnricher 创建新的数据增强器
func NewDataEnricher() *DataEnricher {
	return &DataEnricher{
		logger: logrus.WithField("component", "DataEnricher"),
	}
}

// SetMappingClient 设置映射客户端
func (e *DataEnricher) SetMappingClient(mappingClient api.ProductImportMappingAPI) {
	e.mappingClient = mappingClient
}

// EnrichProductWithMappingBySku 通过SKU查询映射表并填充产品数据
func (e *DataEnricher) EnrichProductWithMappingBySku(
	productData *api.ProductDataDTO,
	sheinProduct *model.SheinProductResponse,
	tenantID, storeID int64,
	inventoryInfo *product.InventoryInfo,
	priceMap map[string]*product.SkuPriceInfo,
	costMap map[string]*product.SkuCostInfo,
) {
	if e.mappingClient == nil {
		e.logger.Warn("映射客户端未设置，跳过数据增强")
		return
	}

	// 遍历SKC和SKU，查询映射关系
	for _, skcInfo := range sheinProduct.SkcInfoList {
		for _, skuInfo := range skcInfo.SkuInfo {
			e.enrichSingleSku(productData, &skuInfo, inventoryInfo, priceMap, costMap)
		}
	}
}

// enrichSingleSku 增强单个SKU数据
func (e *DataEnricher) enrichSingleSku(
	productData *api.ProductDataDTO,
	skuInfo *model.SkuInfo,
	inventoryInfo *product.InventoryInfo,
	priceMap map[string]*product.SkuPriceInfo,
	costMap map[string]*product.SkuCostInfo,
) {
	if e.mappingClient == nil {
		return
	}

	// 查询映射关系
	reqDto := &api.ProductImportMappingGetReqDTO{
		PlatformProductId: skuInfo.SKUCode,
	}

	mappingResp, err := e.mappingClient.GetProductImportMappingByPlatformProductId(reqDto)
	if err != nil {
		e.logger.WithError(err).WithField("sku_code", skuInfo.SKUCode).Debug("未找到映射关系")
		return
	}

	// 填充ASIN等信息
	if mappingResp.ProductId != "" {
		// 这里可以根据需要填充更多信息
		e.logger.WithFields(logrus.Fields{
			"sku_code":   skuInfo.SKUCode,
			"product_id": mappingResp.ProductId,
		}).Debug("成功获取映射关系")
	}
}
