package management

import (
	"fmt"
	"net/http"
	"task-processor/internal/infra/clients/management/api"
	"time"
)

// ActivityProductAPIClient 活动产品数据API客户端实现
type ActivityProductAPIClient struct {
	*ManagementAPIClient
	StoreID int64
}

// BatchSaveActivityProducts 批量保存可报名活动产品数据
func (c *ActivityProductAPIClient) BatchSaveActivityProducts(products []*api.ActivityProductDTO) error {
	if len(products) == 0 {
		return nil
	}

	url := fmt.Sprintf("%s/rpc-api/listing/activity-product/batch-save", c.baseURL)

	platformGroups := c.groupActivityProductsByPlatform(products)

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

func (c *ActivityProductAPIClient) groupActivityProductsByPlatform(products []*api.ActivityProductDTO) map[string][]*api.ActivityProductDTO {
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

func (c *ActivityProductAPIClient) buildActivityProductBatchSaveRequest(platform string, products []*api.ActivityProductDTO) map[string]interface{} {
	activityProductItems := make([]map[string]interface{}, 0, len(products))

	for _, product := range products {
		sitePriceInfoList := make([]map[string]interface{}, 0, len(product.SitePriceInfoList))
		for _, sitePrice := range product.SitePriceInfoList {
			sitePriceInfoList = append(sitePriceInfoList, map[string]interface{}{
				"siteCode":    sitePrice.SiteCode,
				"salePrice":   sitePrice.SalePrice,
				"currency":    sitePrice.Currency,
				"isAvailable": sitePrice.IsAvailable,
			})
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

	var tenantID int64
	var region string
	if len(products) > 0 {
		tenantID = products[0].TenantID
		region = products[0].Region
	}

	return map[string]interface{}{
		"platform":         platform,
		"tenantId":         tenantID,
		"storeId":          c.StoreID,
		"region":           region,
		"activityProducts": activityProductItems,
		"syncTime":         time.Now().UnixMilli(),
	}
}
