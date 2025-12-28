// Package service 提供SHEIN活动报名辅助服务
package service

import (
	"task-processor/internal/common/management/api"
	"task-processor/internal/common/shein/api/marketing"
	"task-processor/internal/common/shein/utils"
)

// ActivityRegistrationHelper 活动报名辅助器
type ActivityRegistrationHelper struct {
	calculator *CostProfitCalculator
}

// NewActivityRegistrationHelper 创建活动报名辅助器
func NewActivityRegistrationHelper() *ActivityRegistrationHelper {
	return &ActivityRegistrationHelper{
		calculator: NewCostProfitCalculator(),
	}
}

// BuildActivityConfig 构建活动配置
func (h *ActivityRegistrationHelper) BuildActivityConfig(products []marketing.SkcInfo) []marketing.ActivityConfig {
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
			ActStock:          utils.CalculateActivityStock(product.Stock),
			DropRate:          utils.GetDefaultDropRate(),
			ReservedActStock:  utils.CalculateReservedStock(product.Stock),
			SitePriceInfoList: sitePriceInfoList,
		}

		configList = append(configList, config)
	}

	return configList
}

// ConvertToRegistrationRecords 转换为报名记录格式
func (h *ActivityRegistrationHelper) ConvertToRegistrationRecords(products []marketing.SkcInfo, tenantID, storeID int64, activityID, activityName string) []*api.ActivityRegistrationDTO {
	registrationRecords := make([]*api.ActivityRegistrationDTO, 0, len(products))

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
			ActStock:           utils.CalculateActivityStock(product.Stock),
			DropRate:           utils.GetDefaultDropRate(),
			ReservedActStock:   utils.CalculateReservedStock(product.Stock),
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
		costPrice, profitRate := h.calculator.CalculateCostAndProfit(product)
		record.CostPrice = costPrice
		record.ProfitRate = profitRate

		registrationRecords = append(registrationRecords, record)
	}

	return registrationRecords
}
