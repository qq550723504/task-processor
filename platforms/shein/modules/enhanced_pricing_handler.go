// Package modules 提供增强的SHEIN核价处理器，保持原有接口和逻辑。
package modules

import (
	"context"
	"fmt"
	"time"

	"task-processor/common/management"
	managementapi "task-processor/common/management/api"
	"task-processor/common/pricing/core"
	shops "task-processor/platforms/shein/client"
	sheinapi "task-processor/platforms/shein/client/api"
	"task-processor/platforms/shein/client/api/pricing"

	"github.com/sirupsen/logrus"
)

// EnhancedAutoPricingHandler 增强的自动核价处理器，保持原有逻辑
type EnhancedAutoPricingHandler struct {
	shopClientMgr *shops.ClientManager
	storeClient   *management.ClientManager
	pricingEngine *core.PricingEngine // 使用通用核价引擎
}

// NewEnhancedAutoPricingHandler 创建新的增强自动核价处理器
func NewEnhancedAutoPricingHandler(shopClientMgr *shops.ClientManager, storeClient *management.ClientManager) *EnhancedAutoPricingHandler {
	return &EnhancedAutoPricingHandler{
		shopClientMgr: shopClientMgr,
		storeClient:   storeClient,
		pricingEngine: core.NewPricingEngine(storeClient),
	}
}

// Start 启动自动核价任务 (保持原有接口)
func (h *EnhancedAutoPricingHandler) Start(ctx context.Context, interval time.Duration) {
	go func() {
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
				return
			}
		}
	}()
}

// performAutoPricing 执行自动核价操作 (保持原有逻辑)
func (h *EnhancedAutoPricingHandler) performAutoPricing() {
	logrus.Info("开始执行SHEIN增强自动核价任务")

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

		if storeInfo.EnableAutoPrice != nil && !*storeInfo.EnableAutoPrice {
			logrus.Infof("店铺 %s 未启用自动核价功能，跳过", storeInfo.Name)
			continue
		}

		// 获取待核价的产品列表 (保持原有逻辑)
		products, err := h.getPendingPricingProducts(tenantShop.TenantID, tenantShop.ShopID, storeInfo)
		if err != nil {
			logrus.Errorf("获取待核价产品失败: %v", err)
			continue
		}

		if len(products) == 0 {
			logrus.Infof("店铺 %s 没有待核价的产品", storeInfo.Name)
			continue
		}

		logrus.Infof("店铺 %s 获取到 %d 个待核价产品", storeInfo.Name, len(products))

		// 处理每个产品的核价逻辑 (保持原有逻辑)
		for _, product := range products {
			h.processProductPricing(tenantShop.TenantID, tenantShop.ShopID, storeInfo, product)
		}
	}

	logrus.Info("SHEIN增强自动核价任务执行完成")
}

// processProductPricing 处理单个产品的核价逻辑 (保持原有逻辑，使用通用引擎)
func (h *EnhancedAutoPricingHandler) processProductPricing(tenantID, storeID int64, storeInfo *managementapi.StoreRespDTO, product pricing.BargainPageData) {
	// 获取店铺API客户端
	client, err := h.shopClientMgr.GetClient(tenantID, storeID, storeInfo)
	if err != nil {
		logrus.Errorf("获取店铺API客户端失败: %v", err)
		return
	}

	// 获取自动核价规则 (使用通用引擎)
	rules, err := h.getAutoPriceRules(tenantID, storeID)
	if err != nil {
		logrus.Errorf("获取自动核价规则失败: %v", err)
		logrus.Infof("产品 %s 核价跳过: 无法获取自动核价规则", product.BargainSn)
		return
	}

	// 处理议价数据并决定是否接受报价 (保持原有逻辑)
	action, reason, batchReq := h.evaluatePricingWithRules(&product, rules)

	// 如果有处理请求，则调用批量处理成本讨论接口
	if batchReq != nil && action != "skip" {
		if err := h.handleCostDiscuss(client, batchReq); err != nil {
			logrus.Errorf("处理成本讨论失败: %v", err)
		}
	}

	logrus.Infof("产品 %s 核价完成: %s", product.BargainSn, reason)
}

// getAutoPriceRules 获取自动核价规则 (使用通用引擎)
func (h *EnhancedAutoPricingHandler) getAutoPriceRules(tenantID, storeID int64) ([]managementapi.PricingRuleRespDTO, error) {
	pricingRule, err := h.pricingEngine.GetPricingRule(tenantID, storeID)
	if err != nil {
		return nil, fmt.Errorf("获取定价规则失败: %w", err)
	}

	// 将单个规则转换为规则数组
	rules := []managementapi.PricingRuleRespDTO{}
	if pricingRule != nil {
		rules = append(rules, *pricingRule)
	}

	return rules, nil
}

// evaluatePricingWithRules 根据规则评估定价是否合理 (保持原有逻辑)
func (h *EnhancedAutoPricingHandler) evaluatePricingWithRules(bargainData *pricing.BargainPageData, rules []managementapi.PricingRuleRespDTO) (action string, reason string, batchReq *pricing.BatchHandleCostDiscussRequest) {
	// 检查是否有SKU成本价数据
	if len(bargainData.SkuCostPrices) == 0 {
		return "skip", "无成本价数据", nil
	}

	// 检查所有SKU是否都符合通过条件 (使用通用引擎)
	allSKUsPass, shouldSkip, err := h.checkAllSKUsPassCondition(bargainData.SkuCostPrices, rules)
	if err != nil {
		logrus.Errorf("检查SKU通过条件时出错: %v", err)
		batchReq := &pricing.BatchHandleCostDiscussRequest{
			ConfirmInfos: &[]pricing.ConfirmInfo{
				{
					DiscussAuditType: 2,
					DiscussSn:        bargainData.BargainSn,
					DocumentSn:       bargainData.DocumentSn,
				},
			},
		}
		return "reject", fmt.Sprintf("检查SKU通过条件时出错: %v", err), batchReq
	}

	if shouldSkip {
		return "skip", "没有有效的SKU可以处理", nil
	}

	if allSKUsPass {
		batchReq := &pricing.BatchHandleCostDiscussRequest{
			ConfirmInfos: &[]pricing.ConfirmInfo{
				{
					DiscussAuditType: 1,
					DiscussSn:        bargainData.BargainSn,
					DocumentSn:       bargainData.DocumentSn,
				},
			},
		}
		return "accept", "所有SKU都符合通过条件", batchReq
	} else {
		batchReq := &pricing.BatchHandleCostDiscussRequest{
			ConfirmInfos: &[]pricing.ConfirmInfo{
				{
					DiscussAuditType: 2,
					DiscussSn:        bargainData.BargainSn,
					DocumentSn:       bargainData.DocumentSn,
				},
			},
		}
		return "reject", "有SKU不符合通过条件", batchReq
	}
}

// checkAllSKUsPassCondition 检查所有SKU是否都符合通过条件 (使用通用引擎)
func (h *EnhancedAutoPricingHandler) checkAllSKUsPassCondition(skus []pricing.SkuCostPrice, rules []managementapi.PricingRuleRespDTO) (bool, bool, error) {
	passedSKUCount := 0
	totalSKUCount := 0

	for _, sku := range skus {
		if len(sku.CostPriceHistories) == 0 {
			continue
		}

		totalSKUCount++

		// 使用通用引擎获取产品导入映射
		respDto, err := h.pricingEngine.GetProductImportMappingByPlatformID(sku.SkuCode)
		if err != nil {
			logrus.Errorf("获取产品导入映射关系失败: %v", err)
			return false, true, nil
		}

		// 使用通用引擎计算原始成本价
		originPrice := h.pricingEngine.CalculateOriginCostPrice(respDto, sku.CostPriceHistories[0].CostPrice)
		suggestPrice := sku.SuggestCostPrice

		if originPrice == 0 {
			logrus.Warnf("[%s] 获取原始价格失败, 跳过", sku.SkuCode)
			continue
		}

		// 使用通用引擎获取自动核价
		passPrice := h.pricingEngine.GetAutoPrice(originPrice, rules)

		if suggestPrice < passPrice {
			logrus.Debugf("SKU: %s, 建议价格: %.2f, 低于通过价格: %.2f, 不符合条件",
				sku.SkuCode, suggestPrice, passPrice)
			return false, false, nil
		}

		passedSKUCount++
	}

	if totalSKUCount == 0 {
		return false, true, nil
	}

	return passedSKUCount == totalSKUCount, false, nil
}

// 以下方法保持原有逻辑不变
func (h *EnhancedAutoPricingHandler) getPendingPricingProducts(tenantID, shopID int64, storeInfo *managementapi.StoreRespDTO) ([]pricing.BargainPageData, error) {
	var allProducts []pricing.BargainPageData

	shopClient, err := h.shopClientMgr.GetClient(tenantID, shopID, storeInfo)
	if err != nil {
		logrus.Errorf("获取租户%d店铺%d的API客户端失败: %v", tenantID, shopID, err)
		return nil, err
	}

	logrus.Infof("成功获取租户%d店铺%d的API客户端", tenantID, shopID)

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

		allProducts = append(allProducts, response.Info.Data...)

		if len(response.Info.Data) < pageSize {
			break
		}
		pageNum++
	}

	return allProducts, nil
}

func (h *EnhancedAutoPricingHandler) handleCostDiscuss(client sheinapi.APIClient, req *pricing.BatchHandleCostDiscussRequest) error {
	response, err := client.BatchHandleCostDiscuss(req)
	if err != nil {
		return fmt.Errorf("调用批量处理成本讨论接口失败: %w", err)
	}

	if response.Code != "0" {
		return fmt.Errorf("批量处理成本讨论接口返回错误: %s", response.Msg)
	}

	logrus.Infof("成功处理成本讨论，成功处理数量: %d", response.Info.SuccessCount)
	return nil
}
