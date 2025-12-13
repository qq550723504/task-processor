// Package shein 提供SHEIN活动报名辅助方法
package shein

import (
	"task-processor/common/management/api"
	"task-processor/platforms/shein/client/api/marketing"
)

// buildActivityConfig 构建活动配置
func (s *ActivityRegistrationService) buildActivityConfig(products []marketing.SkcInfo) []marketing.ActivityConfig {
	configList := make([]marketing.ActivityConfig, 0, len(products))

	for _, product := range products {
		// 转换站点价格信息
		sitePriceInfoList := make([]marketing.ActivitySitePriceInfo, 0, len(product.SitePriceInfoList))
		for _, sitePrice := range product.SitePriceInfoList {
			sitePriceInfoList = append(sitePriceInfoList, marketing.ActivitySitePriceInfo{
				SiteCode:    sitePrice.SiteCode,
				SalePrice:   sitePrice.SalePrice,
				Currency:    sitePrice.Currency,
				IsAvailable: sitePrice.IsAvailable,
			})
		}

		config := marketing.ActivityConfig{
			Skc:               product.Skc,
			ActStock:          calculateActivityStock(product.Stock),
			DropRate:          getDefaultDropRate(),
			ReservedActStock:  calculateReservedStock(product.Stock),
			SitePriceInfoList: sitePriceInfoList,
		}

		configList = append(configList, config)
	}

	return configList
}

// convertToRegistrationRecords 转换为报名记录格式
func (s *ActivityRegistrationService) convertToRegistrationRecords(products []marketing.SkcInfo, tenantID, storeID int64, activityID, activityName string) []*api.ActivityRegistrationDTO {
	registrationRecords := make([]*api.ActivityRegistrationDTO, 0, len(products))
	calculator := NewCostProfitCalculator()

	for _, product := range products {
		// 转换站点价格信息
		sitePriceList := make([]api.ActivityRegistrationSitePriceDTO, 0, len(product.SitePriceInfoList))
		for _, sitePrice := range product.SitePriceInfoList {
			sitePriceList = append(sitePriceList, api.ActivityRegistrationSitePriceDTO{
				SiteCode:    sitePrice.SiteCode,
				SalePrice:   sitePrice.SalePrice,
				Currency:    sitePrice.Currency,
				IsAvailable: sitePrice.IsAvailable,
			})
		}

		record := &api.ActivityRegistrationDTO{
			SKC:                product.Skc,
			GoodsName:          product.GoodsName,
			Image:              product.Image,
			SupplierNo:         product.SupplierNo,
			ActStock:           calculateActivityStock(product.Stock),
			DropRate:           getDefaultDropRate(),
			ReservedActStock:   calculateReservedStock(product.Stock),
			SitePriceInfoList:  sitePriceList,
			RegistrationStatus: 1, // 1:已报名
			Platform:           "SHEIN",
			TenantID:           tenantID,
			StoreID:            storeID,
			Region:             "US", // 默认区域
			ActivityID:         activityID,
			ActivityName:       activityName,
		}

		// 计算并添加成本价和利润率
		calculator.EnrichRegistrationWithCostProfit(record, product)

		registrationRecords = append(registrationRecords, record)
	}

	return registrationRecords
}

// calculateActivityStock 计算活动库存（默认为总库存的80%）
func calculateActivityStock(totalStock int) int {
	if totalStock <= 0 {
		return 0
	}
	actStock := int(float64(totalStock) * 0.8)
	if actStock < 1 {
		actStock = 1
	}
	return actStock
}

// getDefaultDropRate 获取默认降价幅度（20%）
func getDefaultDropRate() int {
	return 20
}

// calculateReservedStock 计算预留库存（默认为总库存的10%）
func calculateReservedStock(totalStock int) int {
	if totalStock <= 0 {
		return 0
	}
	reservedStock := int(float64(totalStock) * 0.1)
	if reservedStock < 1 {
		reservedStock = 1
	}
	return reservedStock
}
