// Package impl 活动产品数据API实现
package impl

import (
	"fmt"
	"net/http"
	"task-processor/internal/pkg/management/api"
	"time"
)

// ActivityProductAPIClientImpl 活动产品数据API客户端实现
type ActivityProductAPIClientImpl struct {
	*ManagementAPIClientImpl
	StoreID int64
}

// BatchSaveActivityProducts 批量保存可报名活动产品数据
func (c *ActivityProductAPIClientImpl) BatchSaveActivityProducts(products []*api.ActivityProductDTO) error {
	if len(products) == 0 {
		return nil
	}

	// 使用新的 RPC API 路径
	url := fmt.Sprintf("%s/rpc-api/listing/activity-product/batch-save", c.baseURL)

	// 按平台分组
	platformGroups := c.groupActivityProductsByPlatform(products)

	// 按平台分别调用
	for platform, groupProducts := range platformGroups {
		reqBody := c.buildActivityProductBatchSaveRequest(platform, groupProducts)

		var result APIResponse
		if err := c.apiRequest(http.MethodPost, url, reqBody, &result); err != nil {
			return fmt.Errorf("批量保存平台 %s 的活动产品失败: %w", platform, err)
		}

		if err := c.ProcessAPIResponse(&result, 0); err != nil {
			return fmt.Errorf("处理平台 %s 的API响应失败: %w", platform, err)
		}
	}

	return nil
}

// groupActivityProductsByPlatform 按平台分组活动产品
func (c *ActivityProductAPIClientImpl) groupActivityProductsByPlatform(products []*api.ActivityProductDTO) map[string][]*api.ActivityProductDTO {
	platformGroups := make(map[string][]*api.ActivityProductDTO)
	for _, product := range products {
		platform := product.Platform
		if platform == "" {
			platform = "UNKNOWN"
		}
		platformGroups[platform] = append(platformGroups[platform], product)
	}
	return platformGroups
}

// buildActivityProductBatchSaveRequest 构建活动产品批量保存请求体
func (c *ActivityProductAPIClientImpl) buildActivityProductBatchSaveRequest(platform string, products []*api.ActivityProductDTO) map[string]interface{} {
	activityProductItems := make([]map[string]interface{}, 0, len(products))

	for _, product := range products {
		// 构建站点价格信息列表
		sitePriceInfoList := make([]map[string]interface{}, 0, len(product.SitePriceInfoList))
		for _, sitePrice := range product.SitePriceInfoList {
			sitePriceInfo := map[string]interface{}{
				"siteCode":    sitePrice.SiteCode,
				"salePrice":   sitePrice.SalePrice,
				"currency":    sitePrice.Currency,
				"isAvailable": sitePrice.IsAvailable,
			}
			sitePriceInfoList = append(sitePriceInfoList, sitePriceInfo)
		}

		item := map[string]interface{}{
			"skc":                 product.SKC,
			"goodsName":           product.GoodsName,
			"image":               product.Image,
			"supplierNo":          product.SupplierNo,
			"stock":               product.Stock,
			"supplyPrice":         product.SupplyPrice,
			"supplyPriceCurrency": product.SupplyPriceCurrency,
			"isConfigured":        product.IsConfigured,
			"sitePriceInfoList":   sitePriceInfoList,
		}

		// 可选字段
		if product.ActStock > 0 {
			item["actStock"] = product.ActStock
		}
		if product.DropRate > 0 {
			item["dropRate"] = product.DropRate
		}
		if product.ReservedActStock > 0 {
			item["reservedActStock"] = product.ReservedActStock
		}
		if product.State > 0 {
			item["state"] = product.State
		}
		if product.CostPrice > 0 {
			item["costPrice"] = product.CostPrice
		}
		if product.ProfitRate > 0 {
			item["profitRate"] = product.ProfitRate
		}

		activityProductItems = append(activityProductItems, item)
	}

	// 从第一个产品中获取租户ID和区域（所有产品应该属于同一租户和区域）
	var tenantID int64
	var region string
	if len(products) > 0 {
		tenantID = products[0].TenantID
		region = products[0].Region
	}

	reqBody := map[string]interface{}{
		"platform":         platform,
		"tenantId":         tenantID,
		"storeId":          c.StoreID,
		"region":           region,
		"activityProducts": activityProductItems,
		"syncTime":         getCurrentTimestamp(),
	}

	return reqBody
}

// getCurrentTimestamp 获取当前时间戳（毫秒）
func getCurrentTimestamp() int64 {
	return time.Now().UnixMilli()
}
