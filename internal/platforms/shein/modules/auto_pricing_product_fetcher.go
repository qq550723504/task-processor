// Package modules 提供SHEIN平台的自动核价产品获取功能
package modules

import (
	managementapi "task-processor/internal/common/management/api"
	shops "task-processor/internal/common/shein"
	"task-processor/internal/common/shein/api/pricing"

	"github.com/sirupsen/logrus"
)

// AutoPricingProductFetcher 自动核价产品获取器，负责获取待核价的产品列表
type AutoPricingProductFetcher struct {
	shopClientMgr *shops.ClientManager
}

// NewAutoPricingProductFetcher 创建新的自动核价产品获取器
func NewAutoPricingProductFetcher(shopClientMgr *shops.ClientManager) *AutoPricingProductFetcher {
	return &AutoPricingProductFetcher{
		shopClientMgr: shopClientMgr,
	}
}

// GetPendingPricingProducts 获取待核价的产品列表
func (f *AutoPricingProductFetcher) GetPendingPricingProducts(tenantID, shopID int64, storeInfo *managementapi.StoreRespDTO) ([]pricing.BargainPageData, error) {
	var allProducts []pricing.BargainPageData

	// 获取店铺API客户端
	shopClient, err := f.shopClientMgr.GetClient(tenantID, shopID, storeInfo)
	if err != nil {
		logrus.Errorf("获取租户%d店铺%d的API客户端失败: %v", tenantID, shopID, err)
		return nil, err
	}

	logrus.Infof("成功获取租户%d店铺%d的API客户端", tenantID, shopID)

	// 分页获取所有待处理的议价数据（状态1表示待处理）
	pageNum := 1
	const pageSize = 100
	for {
		req := &pricing.PageRequest{
			PageNum:  pageNum,
			PageSize: pageSize,
		}

		response, err := shopClient.BargainPage(req, 1)
		if err != nil {
			logrus.Errorf("获取租户%d店铺%d的议价页面数据失败(页面%d): %v", tenantID, shopID, pageNum, err)
			break
		}

		logrus.Debugf("租户%d店铺%d页面%d获取到%d个议价数据", tenantID, shopID, pageNum, len(response.Info.Data))

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
