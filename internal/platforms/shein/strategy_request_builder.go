// Package shein 提供SHEIN平台的策略请求构建功能
package shein

import (
	"encoding/json"
	"task-processor/internal/common/shein/api/product"
	"task-processor/internal/common/shein/api/warehouse"
	"task-processor/internal/pkg/management/api"

	"github.com/sirupsen/logrus"
)

// StrategyRequestBuilder 策略请求构建器
type StrategyRequestBuilder struct{}

// NewStrategyRequestBuilder 创建策略请求构建器
func NewStrategyRequestBuilder() *StrategyRequestBuilder {
	return &StrategyRequestBuilder{}
}

// BuildOffShelfRequest 构建下架请求
func (b *StrategyRequestBuilder) BuildOffShelfRequest(
	prod *api.ProductDataDTO,
	mappings []*SKUMappingData,
) *product.ShelfOperateRequest {
	// 收集所有 SKC 名称
	skcNames := make(map[string]bool)
	for _, mapping := range mappings {
		if mapping.MappingInfo != nil && mapping.MappingInfo.SKU != "" {
			skcNames[mapping.MappingInfo.SKU] = true
		}
	}

	// 构建 SKC 站点信息列表
	var skcSiteInfos []product.SkcSiteInfo
	for skcName := range skcNames {
		skcSiteInfos = append(skcSiteInfos, product.SkcSiteInfo{
			BusinessModel: 1,
			SubSites:      []product.SubSite{},
			OffSubSites: []product.SubSite{
				{
					SiteAbbr:  "shein-us",
					StoreType: 1,
				},
			},
			SkcName: skcName,
		})
	}

	return &product.ShelfOperateRequest{
		SkcSiteInfos: skcSiteInfos,
		SpuName:      prod.ParentProductID,
	}
}

// BuildOnShelfRequest 构建上架请求
func (b *StrategyRequestBuilder) BuildOnShelfRequest(
	prod *api.ProductDataDTO,
	mappings []*SKUMappingData,
) *product.ShelfOperateRequest {
	// 收集所有 SKC 名称
	skcNames := make(map[string]bool)
	for _, mapping := range mappings {
		if mapping.MappingInfo != nil && mapping.MappingInfo.SKU != "" {
			skcNames[mapping.MappingInfo.SKU] = true
		}
	}

	// 构建 SKC 站点信息列表
	var skcSiteInfos []product.SkcSiteInfo
	for skcName := range skcNames {
		skcSiteInfos = append(skcSiteInfos, product.SkcSiteInfo{
			BusinessModel: 1,
			SubSites: []product.SubSite{
				{
					SiteAbbr:  "shein-us",
					StoreType: 1,
				},
			},
			OffSubSites: []product.SubSite{},
			SkcName:     skcName,
		})
	}

	return &product.ShelfOperateRequest{
		SkcSiteInfos: skcSiteInfos,
		SpuName:      prod.PlatformProductID,
	}
}

// BuildInventoryUpdateRequestFromAttributes 从 Attributes 和仓库信息构建库存更新请求
func (b *StrategyRequestBuilder) BuildInventoryUpdateRequestFromAttributes(
	attributesJSON string,
	warehouseResp *warehouse.WarehouseResponse,
	platformSKU string,
	oldStock int,
	newStock int,
) *product.InventoryUpdateRequest {
	if attributesJSON == "" {
		return nil
	}

	var skcList []SKCInfo
	if err := json.Unmarshal([]byte(attributesJSON), &skcList); err != nil {
		logrus.WithError(err).Error("解析 Attributes 失败")
		return nil
	}

	// 获取第一个可用仓库代码
	var warehouseCode string
	if len(warehouseResp.Data) > 0 {
		warehouseCode = warehouseResp.Data[0].WarehouseCode
	}

	var skcUpdates []product.SkcInventoryUpdate

	// 遍历所有 SKC
	for _, skc := range skcList {
		var skuUpdates []product.SkuInventoryUpdate

		// 遍历 SKC 下的所有 SKU
		for _, sku := range skc.SKUInfo {
			// 找到匹配的 SKU（通过 MappingInfo.SKU 匹配）
			if sku.MappingInfo != nil && sku.MappingInfo.SKU == platformSKU {
				// 构建仓库库存更新信息
				var warehouseUpdates []product.WarehouseInventoryUpdate
				warehouseUpdates = append(warehouseUpdates, product.WarehouseInventoryUpdate{
					MerchantWarehouseCode:    warehouseCode,
					BeforeChangeInventoryNum: oldStock,
					AfterChangeInventoryNum:  newStock,
				})

				if len(warehouseUpdates) > 0 {
					skuUpdates = append(skuUpdates, product.SkuInventoryUpdate{
						SkuCode:           sku.SKUCode,
						DeliveryMode:      1,
						WarehouseInfoList: warehouseUpdates,
					})
				}
			}
		}

		// 如果该 SKC 有需要更新的 SKU
		if len(skuUpdates) > 0 {
			skcUpdates = append(skcUpdates, product.SkcInventoryUpdate{
				SkcCode: skc.SKCCode,
				SkcName: platformSKU,
				SkuInfo: skuUpdates,
			})
		}
	}

	if len(skcUpdates) == 0 {
		return nil
	}

	return &product.InventoryUpdateRequest{
		SkcInfo: skcUpdates,
	}
}
