// Package modules 提供SHEIN平台的自动核价规则评估功能
package modules

import (
	"fmt"

	"task-processor/internal/common/management"
	managementapi "task-processor/internal/common/management/api"
	"task-processor/internal/common/shein/api/pricing"

	"github.com/sirupsen/logrus"
)

// AutoPricingRuleEvaluator 自动核价规则评估器，负责评估产品是否符合核价规则
type AutoPricingRuleEvaluator struct {
	storeClient     *management.ClientManager
	priceCalculator *AutoPricingCalculator
}

// NewAutoPricingRuleEvaluator 创建新的自动核价规则评估器
// 参数:
//   - storeClient: 存储客户端管理器
//
// 返回值:
//   - *AutoPricingRuleEvaluator: 规则评估器实例
func NewAutoPricingRuleEvaluator(storeClient *management.ClientManager) *AutoPricingRuleEvaluator {
	return &AutoPricingRuleEvaluator{
		storeClient:     storeClient,
		priceCalculator: NewAutoPricingCalculator(),
	}
}

// EvaluateProductPricing 评估产品核价
// 参数:
//   - tenantID: 租户ID
//   - storeID: 店铺ID
//   - product: 产品数据
//
// 返回值:
//   - action: 操作类型 (accept/reject/skip)
//   - reason: 操作原因
//   - batchReq: 批量处理请求
func (e *AutoPricingRuleEvaluator) EvaluateProductPricing(tenantID, storeID int64, product interface{}) (action string, reason string, batchReq interface{}) {
	// 类型断言获取具体的产品数据
	bargainData, ok := product.(pricing.BargainPageData)
	if !ok {
		return "skip", "产品数据类型错误", nil
	}

	// 获取自动核价规则
	rules, err := e.getAutoPriceRules(tenantID, storeID)
	if err != nil {
		logrus.Errorf("获取自动核价规则失败: %v", err)
		return "skip", fmt.Sprintf("无法获取自动核价规则: %v", err), nil
	}

	// 处理议价数据并决定是否接受报价
	return e.evaluatePricingWithRules(&bargainData, rules)
}

// getAutoPriceRules 获取自动核价规则
func (e *AutoPricingRuleEvaluator) getAutoPriceRules(tenantID, storeID int64) ([]managementapi.PricingRuleRespDTO, error) {
	// 调用管理端API获取定价规则
	pricingRuleAPI := e.storeClient.GetPricingRuleClient()
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
func (e *AutoPricingRuleEvaluator) evaluatePricingWithRules(bargainData *pricing.BargainPageData, rules []managementapi.PricingRuleRespDTO) (action string, reason string, batchReq *pricing.BatchHandleCostDiscussRequest) {
	// 检查是否有SKU成本价数据
	if len(bargainData.SkuCostPrices) == 0 {
		return "skip", "无成本价数据", nil
	}

	// 检查所有SKU是否都符合通过条件
	allSKUsPass, shouldSkip, err := e.checkAllSKUsPassCondition(bargainData.SkuCostPrices, rules)
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
func (e *AutoPricingRuleEvaluator) checkAllSKUsPassCondition(skus []pricing.SkuCostPrice, rules []managementapi.PricingRuleRespDTO) (bool, bool, error) {
	mappingClient := e.storeClient.GetProductImportMappingClient()
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
		passPrice := e.priceCalculator.GetAutoPrice(originPrice, rules)

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
