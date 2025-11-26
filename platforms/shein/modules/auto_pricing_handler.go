package modules

import (
	"context"
	"fmt"
	"time"

	"task-processor/common/management"
	managementapi "task-processor/common/management/api" // 使用别名避免冲突
	shops "task-processor/common/shein"                  // 导入shein包并使用shops别名
	sheinapi "task-processor/common/shein/api"           // 使用别名避免冲突
	"task-processor/common/shein/api/pricing"

	"github.com/sirupsen/logrus"
)

// AutoPricingHandler 自动核价处理器
type AutoPricingHandler struct {
	shopClientMgr *shops.ClientManager
	storeClient   *management.ClientManager
}

// NewAutoPricingHandler 创建新的自动核价处理器
func NewAutoPricingHandler(shopClientMgr *shops.ClientManager, storeClient *management.ClientManager) *AutoPricingHandler {
	return &AutoPricingHandler{
		shopClientMgr: shopClientMgr,
		storeClient:   storeClient,
	}
}

// Start 启动自动核价任务
func (h *AutoPricingHandler) Start(ctx context.Context, interval time.Duration) {
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

		// 2. 处理每个产品的核价逻辑
		for _, product := range products {
			h.processProductPricing(tenantShop.TenantID, tenantShop.ShopID, storeInfo, product)
		}

	}

	logrus.Info("自动核价任务执行完成")
}

// getPendingPricingProducts 获取待核价的产品列表
func (h *AutoPricingHandler) getPendingPricingProducts(tenantID, shopID int64, storeInfo *managementapi.StoreRespDTO) ([]pricing.BargainPageData, error) {

	var allProducts []pricing.BargainPageData

	// 获取店铺API客户端
	shopClient, err := h.shopClientMgr.GetClient(tenantID, shopID, storeInfo)
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

// processProductPricing 处理单个产品的核价逻辑
func (h *AutoPricingHandler) processProductPricing(tenantID, storeID int64, storeInfo *managementapi.StoreRespDTO, product pricing.BargainPageData) {

	// 1. 获取店铺API客户端
	client, err := h.shopClientMgr.GetClient(tenantID, storeID, storeInfo)
	if err != nil {
		logrus.Errorf("获取店铺API客户端失败: %v", err)
		return
	}

	// 2. 获取自动核价规则
	rules, err := h.getAutoPriceRules(tenantID, storeID)
	if err != nil {
		logrus.Errorf("获取自动核价规则失败: %v", err)
		// 当无法获取自动核价规则时，跳过核价处理
		logrus.Infof("产品 %s 核价跳过: 无法获取自动核价规则", product.BargainSn)
		return
	}

	// 3. 处理议价数据并决定是否接受报价
	action, reason, batchReq := h.evaluatePricingWithRules(&product, rules)

	// 4. 如果有处理请求，则调用批量处理成本讨论接口
	// 只有当action不是"skip"时才处理
	if batchReq != nil && action != "skip" {
		if err := h.handleCostDiscuss(client, batchReq); err != nil {
			logrus.Errorf("处理成本讨论失败: %v", err)
		}
	}

	logrus.Infof("产品 %s 核价完成: %s", product.BargainSn, reason)

}

// getAutoPriceRules 获取自动核价规则
func (h *AutoPricingHandler) getAutoPriceRules(tenantID, storeID int64) ([]managementapi.PricingRuleRespDTO, error) {
	// 调用管理端API获取定价规则
	pricingRuleAPI := h.storeClient.GetPricingRuleClient()
	// 使用managementapi包中的PricingRuleReqDTO类型
	reqDto := &managementapi.PricingRuleReqDTO{StoreID: &storeID, TenantID: tenantID}
	rule, err := pricingRuleAPI.GetPricingRule(reqDto)
	if err != nil {
		return nil, fmt.Errorf("获取定价规则失败: %w", err)
	}

	// 将单个规则转换为规则数组
	rules := []managementapi.PricingRuleRespDTO{}
	if rule != nil {
		rules = append(rules, *rule)
	}

	return rules, nil
}

// evaluatePricingWithRules 根据规则评估定价是否合理
func (h *AutoPricingHandler) evaluatePricingWithRules(bargainData *pricing.BargainPageData, rules []managementapi.PricingRuleRespDTO) (action string, reason string, batchReq *pricing.BatchHandleCostDiscussRequest) {
	// 检查是否有SKU成本价数据
	if len(bargainData.SkuCostPrices) == 0 {
		return "skip", "无成本价数据", nil
	}

	// 检查所有SKU是否都符合通过条件
	allSKUsPass, shouldSkip, err := h.checkAllSKUsPassCondition(bargainData.SkuCostPrices, rules)
	if err != nil {
		logrus.Errorf("检查SKU通过条件时出错: %v", err)
		// 当无法检查SKU条件时，默认拒绝报价而不是返回错误
		batchReq := &pricing.BatchHandleCostDiscussRequest{
			ConfirmInfos: &[]pricing.ConfirmInfo{
				{
					DiscussAuditType: 2, // 2表示拒绝
					DiscussSn:        bargainData.BargainSn,
					DocumentSn:       bargainData.DocumentSn,
				},
			},
		}
		return "reject", fmt.Sprintf("检查SKU通过条件时出错: %v", err), batchReq
	}

	// 如果应该跳过处理
	if shouldSkip {
		return "skip", "没有有效的SKU可以处理", nil
	}

	if allSKUsPass {
		// 所有SKU都通过，同意报价
		batchReq := &pricing.BatchHandleCostDiscussRequest{
			ConfirmInfos: &[]pricing.ConfirmInfo{
				{
					DiscussAuditType: 1, // 1表示接受
					DiscussSn:        bargainData.BargainSn,
					DocumentSn:       bargainData.DocumentSn,
				},
			},
		}
		return "accept", "所有SKU都符合通过条件", batchReq
	} else {
		// 有SKU不通过，拒绝报价
		batchReq := &pricing.BatchHandleCostDiscussRequest{
			ConfirmInfos: &[]pricing.ConfirmInfo{
				{
					DiscussAuditType: 2, // 2表示拒绝
					DiscussSn:        bargainData.BargainSn,
					DocumentSn:       bargainData.DocumentSn,
				},
			},
		}
		return "reject", "有SKU不符合通过条件", batchReq
	}
}

// checkAllSKUsPassCondition 检查所有SKU是否都符合通过条件
// 返回值: (是否通过, 是否跳过处理, 错误)
func (h *AutoPricingHandler) checkAllSKUsPassCondition(skus []pricing.SkuCostPrice, rules []managementapi.PricingRuleRespDTO) (bool, bool, error) {
	mappingClient := h.storeClient.GetProductImportMappingClient()
	passedSKUCount := 0
	totalSKUCount := 0

	for _, sku := range skus {
		if len(sku.CostPriceHistories) == 0 {
			continue
		}

		totalSKUCount++

		reqDto := &managementapi.ProductImportMappingGetReqDTO{PlatformProductId: sku.SkuCode}

		respDto, err := mappingClient.GetProductImportMappingByPlatformProductId(reqDto)
		if err != nil {
			logrus.Errorf("获取产品导入映射关系失败: %v", err)
			// 当无法获取产品导入映射关系时，应该跳过整个核价过程
			return false, true, nil
		}

		// 修复：正确处理originPrice变量作用域
		var originPrice float64
		if respDto.CostPrice != nil {
			originPrice = *respDto.CostPrice
		} else {
			// 确保不会除以零
			if respDto.SalePriceMultiplier != nil {
				// SalePriceMultiplier现在是float64类型，直接使用
				if *respDto.SalePriceMultiplier != 0 {
					originPrice = sku.CostPriceHistories[0].CostPrice / *respDto.SalePriceMultiplier
				} else {
					originPrice = 0
				}
			} else {
				originPrice = 0
			}
		}
		suggestPrice := sku.SuggestCostPrice

		if originPrice == 0 {
			logrus.Warnf("[%s] 获取原始价格失败, 跳过", sku.SkuCode)
			// 继续检查下一个SKU，而不是终止整个检查过程
			continue
		}

		// 获取通过价格
		passPrice := h.getAutoPrice(originPrice, rules)

		if suggestPrice < passPrice {
			logrus.Debugf("SKU: %s, 建议价格: %.2f, 低于通过价格: %.2f, 不符合条件",
				sku.SkuCode, suggestPrice, passPrice)
			// 有一个SKU不通过就返回false
			return false, false, nil
		}

		// 当前SKU通过检查
		passedSKUCount++
	}

	// 如果没有任何SKU被检查，则返回跳过处理
	if totalSKUCount == 0 {
		return false, true, nil
	}

	// 只有当所有被检查的SKU都通过时才返回true
	// 如果有任何一个SKU不通过，已经在上面返回false了
	return passedSKUCount == totalSKUCount, false, nil
}

// getAutoPrice 获取自动核价
func (h *AutoPricingHandler) getAutoPrice(originPrice float64, rules []managementapi.PricingRuleRespDTO) float64 {
	for _, rule := range rules {
		// 判断是否在规则范围内
		if rule.PriceMin != nil && rule.PriceMax != nil &&
			originPrice >= *rule.PriceMin && originPrice < *rule.PriceMax {
			return h.applyRule(originPrice, rule)
		}
	}
	return originPrice // 没有匹配规则则原价
}

// applyRule 应用自动核价规则
func (h *AutoPricingHandler) applyRule(originPrice float64, rule managementapi.PricingRuleRespDTO) float64 {
	if rule.RuleValue == nil {
		return originPrice
	}

	switch rule.RuleType {
	case "fixed":
		// 固定加价
		return originPrice + *rule.RuleValue
	case "percent":
		// 加价百分比
		return originPrice * (1 + *rule.RuleValue)
	case "multiple":
		// 倍数
		return *rule.RuleValue * originPrice
	case "discount":
		// 折扣率
		return originPrice * (1 - *rule.RuleValue)
	case "fixed_price":
		// 固定价格
		return *rule.RuleValue
	}
	return originPrice
}

// handleCostDiscuss 处理成本讨论
func (h *AutoPricingHandler) handleCostDiscuss(client sheinapi.APIClient, req *pricing.BatchHandleCostDiscussRequest) error {
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
