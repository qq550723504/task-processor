// Package repo 提供SHEIN平台的自动核价产品获取功能
package repo

import (
	managementapi "task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/api/pricing"

	"github.com/sirupsen/logrus"
)

// AutoPricingProductFetcher 自动核价产品获取器，负责获取待核价的产品列表
type AutoPricingProductFetcher struct {
	pricingAPI *PricingAPI
}

// NewAutoPricingProductFetcher 创建新的自动核价产品获取器
func NewAutoPricingProductFetcher(pricingAPI *PricingAPI) *AutoPricingProductFetcher {
	return &AutoPricingProductFetcher{
		pricingAPI: pricingAPI,
	}
}

// GetPendingPricingProducts 获取待核价的产品列表
func (f *AutoPricingProductFetcher) GetPendingPricingProducts(shopID int64, storeInfo *managementapi.StoreRespDTO) ([]pricing.BargainPageData, error) {
	var allProducts []pricing.BargainPageData

	logrus.Infof("开始获取租户%d店铺%d的待核价产品", storeInfo.TenantID, shopID)

	// 分页获取所有待处理的议价数据（状态1表示待处理）
	pageNum := 1
	const pageSize = 100
	for {
		req := &pricing.PageRequest{
			PageNum:  pageNum,
			PageSize: pageSize,
		}

		response, err := f.pricingAPI.BargainPage(req, 1)
		if err != nil {
			logrus.Errorf("获取租户%d店铺%d的议价页面数据失败(页面%d): %v", storeInfo.TenantID, shopID, pageNum, err)
			break
		}

		logrus.Debugf("租户%d店铺%d页面%d获取到%d个议价数据", storeInfo.TenantID, shopID, pageNum, len(response.Info.Data))

		// 将议价数据转换为定价任务
		allProducts = append(allProducts, response.Info.Data...)

		// 如果当前页数据少于页面大小，说明已经到最后一页
		if len(response.Info.Data) < pageSize {
			break
		}
		pageNum++
	}

	return allProducts, nil
}
