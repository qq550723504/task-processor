// Package shein 提供SHEIN活动产品同步辅助方法
package shein

import (
	"fmt"
	"task-processor/common/management/api"
	"task-processor/platforms/shein/client/api/marketing"

	"github.com/sirupsen/logrus"
)

// fetchAllActivityProducts 获取所有可报名活动的产品
func (s *ActivitySyncService) fetchAllActivityProducts(marketingAPI marketing.MarketingAPI) ([]marketing.SkcInfo, error) {
	var allProducts []marketing.SkcInfo
	pageNum := 1
	pageSize := 100

	for {
		req := &marketing.GetAvailableSkcListRequest{
			PageNum:  pageNum,
			PageSize: pageSize,
		}

		resp, err := marketingAPI.GetAvailableSkcList(req)
		if err != nil {
			return nil, fmt.Errorf("获取第 %d 页活动产品失败: %w", pageNum, err)
		}

		if resp.Info == nil || len(resp.Info.SkcInfoList) == 0 {
			break
		}

		// 直接使用返回的数据结构
		allProducts = append(allProducts, resp.Info.SkcInfoList...)

		logrus.WithFields(logrus.Fields{
			"page":       pageNum,
			"page_count": len(resp.Info.SkcInfoList),
			"total":      len(allProducts),
		}).Info("已获取活动产品页面数据")

		// 如果本页数量小于页大小，说明已经是最后一页
		if len(resp.Info.SkcInfoList) < pageSize {
			break
		}

		pageNum++
	}

	return allProducts, nil
}

// convertToBackendFormat 转换为后端API格式
func (s *ActivitySyncService) convertToBackendFormat(products []marketing.SkcInfo, tenantID, storeID int64) []*api.ActivityProductDTO {
	backendProducts := make([]*api.ActivityProductDTO, 0, len(products))
	calculator := NewCostProfitCalculator()

	for _, product := range products {
		// 转换站点价格信息
		sitePriceList := make([]api.ActivitySitePriceDTO, 0, len(product.SitePriceInfoList))
		for _, sitePrice := range product.SitePriceInfoList {
			sitePriceList = append(sitePriceList, api.ActivitySitePriceDTO{
				SiteCode:    sitePrice.SiteCode,
				SalePrice:   sitePrice.SalePrice,
				Currency:    sitePrice.Currency,
				IsAvailable: sitePrice.IsAvailable,
			})
		}

		backendProduct := &api.ActivityProductDTO{
			SKC:                 product.Skc,
			GoodsName:           product.GoodsName,
			Image:               product.Image,
			SupplierNo:          product.SupplierNo,
			Stock:               product.Stock,
			SupplyPrice:         product.SupplyPrice,
			SupplyPriceCurrency: product.SupplyPriceCurrency,
			IsConfigured:        product.IsConfigured,
			SitePriceInfoList:   sitePriceList,
			Platform:            "SHEIN",
			TenantID:            tenantID,
			StoreID:             storeID,
			Region:              "US", // 默认区域，可以根据需要调整
			State:               1,    // 默认状态为正常
		}

		// 计算并添加成本价和利润率
		calculator.EnrichActivityProductWithCostProfit(backendProduct, product)

		backendProducts = append(backendProducts, backendProduct)
	}

	return backendProducts
}
