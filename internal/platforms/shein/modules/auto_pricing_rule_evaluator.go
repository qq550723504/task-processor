// Package modules 提供SHEIN平台的自动核价规则评估功能
package modules

import (
	"fmt"

	"task-processor/internal/pkg/management"
	managementapi "task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/api/pricing"

	"github.com/sirupsen/logrus"
)

// AutoPricingRuleEvaluator 自动核价规则评估器，负责评估产品是否符合核价规则
type AutoPricingRuleEvaluator struct {
	storeClient     *management.ClientManager
	priceCalculator *AutoPricingCalculator
}

// NewAutoPricingRuleEvaluator 创建新的自动核价规则评估器
func NewAutoPricingRuleEvaluator(storeClient *management.ClientManager) *AutoPricingRuleEvaluator {
	return &AutoPricingRuleEvaluator{
		storeClient:     storeClient,
		priceCalculator: NewAutoPricingCalculator(),
	}
}

// EvaluateProductPricing 评估产品核价
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
	reqDto := &managementapi.PricingRuleReqDTO{StoreID: &storeID, TenantID: tenantID}
	rules, err := pricingRuleAPI.GetPricingRule(reqDto)
	if err != nil {
		return nil, fmt.Errorf("获取定价规则失败: %w", err)
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
			return false, true, nil
		}

		var originPrice float64
		if respDto.CostPrice != nil {
			originPrice = *respDto.CostPrice
		} else {
			if respDto.SalePriceMultiplier != nil {
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
			continue
		}

		// 获取通过价格
		passPrice := e.priceCalculator.GetAutoPrice(originPrice, rules)

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
