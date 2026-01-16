// Package scheduler 提供SHEIN平台调度器相关服务
package scheduler

import (
	"fmt"

	managementapi "task-processor/internal/pkg/management/api"
	managementimpl "task-processor/internal/pkg/management/impl"
	"task-processor/internal/platforms/shein/api/marketing"

	"github.com/sirupsen/logrus"
)

// filterProductsByProfitMargin 根据利润率过滤产品
// 只保留利润率 >= 降价百分比的产品，避免降价后亏本
func (s *activityRegistrationServiceImpl) filterProductsByProfitMargin(
	products []marketing.SkcInfo,
	discountRate float64,
	storeID int64,
) []marketing.SkcInfo {
	if len(products) == 0 {
		return products
	}

	// 获取产品导入映射API客户端
	baseClient := s.managementClient.GetClient()
	productMappingAPI := &managementimpl.ProductImportMappingAPIClientImpl{
		ManagementAPIClientImpl: baseClient,
	}

	filteredProducts := make([]marketing.SkcInfo, 0, len(products))
	filteredCount := 0

	for _, product := range products {
		// 1. 根据SKC查询管理系统获取成本价
		mapping, err := productMappingAPI.GetProductImportMappingByPlatformProductIdAndStore(
			&managementapi.ProductImportMappingGetByPlatformProductIdAndStoreReqDTO{
				PlatformProductId: product.Skc,
				StoreId:           storeID,
			},
		)

		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"skc":   product.Skc,
				"error": err,
			}).Warn("查询产品成本价失败，跳过该产品")
			filteredCount++
			continue
		}

		if mapping == nil || mapping.CostPrice == nil {
			s.logger.WithField("skc", product.Skc).Warn("产品成本价为空，跳过该产品")
			filteredCount++
			continue
		}

		costPrice := *mapping.CostPrice

		// 2. 计算利润率：(售价 - 成本价) / 成本价
		// 使用第一个站点的售价作为参考
		if len(product.SitePriceInfoList) == 0 {
			s.logger.WithField("skc", product.Skc).Warn("产品没有站点价格信息，跳过该产品")
			filteredCount++
			continue
		}

		salePrice := product.SitePriceInfoList[0].SalePrice
		if salePrice <= 0 || costPrice <= 0 {
			s.logger.WithFields(logrus.Fields{
				"skc":        product.Skc,
				"sale_price": salePrice,
				"cost_price": costPrice,
			}).Warn("产品价格异常，跳过该产品")
			filteredCount++
			continue
		}

		profitMargin := (salePrice - costPrice) / costPrice

		// 3. 检查利润率是否 >= 降价百分比
		if profitMargin < discountRate {
			s.logger.WithFields(logrus.Fields{
				"skc":           product.Skc,
				"profit_margin": fmt.Sprintf("%.2f%%", profitMargin*100),
				"discount_rate": fmt.Sprintf("%.2f%%", discountRate*100),
				"sale_price":    salePrice,
				"cost_price":    costPrice,
			}).Info("产品利润率不足，无法支持降价，已过滤")
			filteredCount++
			continue
		}

		// 利润率足够，保留该产品
		filteredProducts = append(filteredProducts, product)
	}

	s.logger.WithFields(logrus.Fields{
		"total_products":    len(products),
		"filtered_products": len(filteredProducts),
		"removed_products":  filteredCount,
	}).Info("利润率过滤完成")

	return filteredProducts
}
