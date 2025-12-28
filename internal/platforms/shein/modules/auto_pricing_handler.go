// Package modules 提供SHEIN平台的自动核价处理功能
package modules

import (
	"context"
	"time"

	"task-processor/internal/common/management"
	managementapi "task-processor/internal/common/management/api"
	shops "task-processor/internal/common/shein"
	"task-processor/internal/common/shein/api/pricing"

	"github.com/sirupsen/logrus"
)

// AutoPricingHandler 自动核价处理器，负责协调自动核价的各个组件
type AutoPricingHandler struct {
	shopClientMgr   *shops.ClientManager
	storeClient     *management.ClientManager
	productFetcher  *AutoPricingProductFetcher
	ruleEvaluator   *AutoPricingRuleEvaluator
	priceCalculator *AutoPricingCalculator
	discussHandler  *AutoPricingDiscussHandler
}

// NewAutoPricingHandler 创建新的自动核价处理器
// 参数:
//   - shopClientMgr: 店铺客户端管理器
//   - storeClient: 存储客户端管理器
//
// 返回值:
//   - *AutoPricingHandler: 自动核价处理器实例
func NewAutoPricingHandler(shopClientMgr *shops.ClientManager, storeClient *management.ClientManager) *AutoPricingHandler {
	return &AutoPricingHandler{
		shopClientMgr:   shopClientMgr,
		storeClient:     storeClient,
		productFetcher:  NewAutoPricingProductFetcher(shopClientMgr),
		ruleEvaluator:   NewAutoPricingRuleEvaluator(storeClient),
		priceCalculator: NewAutoPricingCalculator(),
		discussHandler:  NewAutoPricingDiscussHandler(),
	}
}

// Start 启动自动核价任务
// 参数:
//   - ctx: 上下文，用于控制任务生命周期
//   - interval: 执行间隔
func (h *AutoPricingHandler) Start(ctx context.Context, interval time.Duration) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("自动核价处理器goroutine panic: %v", r)
			}
		}()

		// 立即执行一次
		h.performAutoPricing()

		// 之后按指定间隔执行
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// 执行自动核价
				h.performAutoPricing()
			case <-ctx.Done():
				// 上下文被取消，退出
				logrus.Info("自动核价处理器停止")
				return
			}
		}
	}()
}

// performAutoPricing 执行自动核价操作
func (h *AutoPricingHandler) performAutoPricing() {
	logrus.Info("开始执行SHEIN自动核价任务")

	tenantShops := h.shopClientMgr.GetTenantShopPairs()
	logrus.Infof("获取到 %d 个租户店铺对", len(tenantShops))

	if len(tenantShops) == 0 {
		return
	}

	for _, tenantShop := range tenantShops {
		logrus.Infof("处理租户 %d 的店铺 %d", tenantShop.TenantID, tenantShop.ShopID)

		storeInfo, err := h.storeClient.GetStoreClient().GetStore(tenantShop.ShopID)
		if err != nil {
			logrus.Errorf("获取店铺信息失败: %v", err)
			continue
		}

		logrus.Infof("店铺 %s (ID: %d) 自动核价状态: %v", storeInfo.Name, storeInfo.ID,
			storeInfo.EnableAutoPrice != nil && *storeInfo.EnableAutoPrice)

		if storeInfo.EnableAutoPrice != nil && !*storeInfo.EnableAutoPrice { // 店铺未启用自动核价功能
			logrus.Infof("店铺 %s 未启用自动核价功能，跳过", storeInfo.Name)
			continue
		}

		// 2. 获取待核价的产品列表
		products, err := h.productFetcher.GetPendingPricingProducts(tenantShop.TenantID, tenantShop.ShopID, storeInfo)
		if err != nil {
			logrus.Errorf("获取待核价产品失败: %v", err)
			continue
		}

		if len(products) == 0 {
			logrus.Infof("店铺 %s 没有待核价的产品", storeInfo.Name)
			continue
		}

		logrus.Infof("店铺 %s 获取到 %d 个待核价产品", storeInfo.Name, len(products))

		// 2. 处理每个产品的核价逻辑
		for _, product := range products {
			h.processProductPricing(tenantShop.TenantID, tenantShop.ShopID, storeInfo, product)
		}

	}

	logrus.Info("自动核价任务执行完成")
}

// processProductPricing 处理单个产品的核价逻辑
func (h *AutoPricingHandler) processProductPricing(tenantID, storeID int64, storeInfo interface{}, product interface{}) {
	// 类型断言获取具体的店铺信息
	storeInfoDTO, ok := storeInfo.(*managementapi.StoreRespDTO)
	if !ok {
		logrus.Errorf("店铺信息类型断言失败")
		return
	}

	// 1. 获取店铺API客户端
	client, err := h.shopClientMgr.GetClient(tenantID, storeID, storeInfoDTO)
	if err != nil {
		logrus.Errorf("获取店铺API客户端失败: %v", err)
		return
	}

	// 2. 获取自动核价规则并评估
	action, reason, batchReq := h.ruleEvaluator.EvaluateProductPricing(tenantID, storeID, product)

	// 3. 如果有处理请求，则调用批量处理成本讨论接口
	if batchReq != nil && action != "skip" {
		// 类型断言获取具体的批量请求
		if batchReqDTO, ok := batchReq.(*pricing.BatchHandleCostDiscussRequest); ok {
			if err := h.discussHandler.HandleCostDiscuss(client, batchReqDTO); err != nil {
				logrus.Errorf("处理成本讨论失败: %v", err)
			}
		} else {
			logrus.Errorf("批量请求类型断言失败")
		}
	}

	logrus.Infof("产品核价完成: %s", reason)
}
