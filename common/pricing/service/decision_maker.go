// Package service 提供核价决策制定者的实现。
package service

import (
	"context"
	"fmt"
	"task-processor/common/pricing/model"

	"github.com/sirupsen/logrus"
)

// DefaultPricingDecisionMaker 默认核价决策制定者实现
type DefaultPricingDecisionMaker struct {
	calculator   PricingCalculator
	costProvider CostPriceProvider
	logger       *logrus.Entry
}

// NewDefaultPricingDecisionMaker 创建默认核价决策制定者
func NewDefaultPricingDecisionMaker(calculator PricingCalculator, costProvider CostPriceProvider) *DefaultPricingDecisionMaker {
	return &DefaultPricingDecisionMaker{
		calculator:   calculator,
		costProvider: costProvider,
		logger:       logrus.WithField("component", "DefaultPricingDecisionMaker"),
	}
}

// MakeDecision 制定核价决策
func (d *DefaultPricingDecisionMaker) MakeDecision(ctx context.Context, pricingCtx *model.PricingContext) (*model.PricingResult, error) {
	// 验证上下文
	if err := d.ValidateContext(pricingCtx); err != nil {
		return nil, fmt.Errorf("验证核价上下文失败: %w", err)
	}

	// 创建核价结果
	result := &model.PricingResult{
		ProductID:       pricingCtx.ProductID,
		SkuID:           pricingCtx.SkuID,
		ProductName:     pricingCtx.ProductName,
		CurrentPrice:    pricingCtx.CurrentPrice,
		SuggestPrice:    pricingCtx.SuggestPrice,
		OriginCostPrice: pricingCtx.OriginCostPrice,
	}

	// 检查是否启用自动核价
	if !pricingCtx.IsAutoPricingEnabled() {
		result.SetDecision(model.ActionSkip, "店铺未启用自动核价功能")
		return result, nil
	}

	// 检查是否有有效的成本价
	if !pricingCtx.HasValidCostPrice() {
		result.SetDecision(model.ActionSkip, "无有效的成本价信息")
		return result, nil
	}

	// 计算目标价格
	calculatedPrice, appliedRule, err := d.calculator.CalculatePrice(ctx, pricingCtx.OriginCostPrice, pricingCtx.PricingRules)
	if err != nil {
		d.logger.Errorf("计算价格失败: %v", err)
		result.SetDecision(model.ActionSkip, fmt.Sprintf("价格计算失败: %v", err))
		return result, nil
	}

	result.CalculatedPrice = calculatedPrice
	result.AcceptablePrice = calculatedPrice
	if appliedRule != nil {
		result.AppliedRuleName = appliedRule.Name
		result.AppliedRuleType = string(appliedRule.RuleType)
		if appliedRule.RuleValue != nil {
			result.TargetMargin = *appliedRule.RuleValue * 100 // 转换为百分比
			result.MinMargin = result.TargetMargin
		}
	}

	// 计算利润率
	result.CalculateProfitMargin()

	// 制定决策
	d.makeDecisionLogic(pricingCtx, result)

	d.logger.Infof("商品 %s 核价决策: %s - %s",
		pricingCtx.ProductName, result.Action, result.Reason)

	return result, nil
}

// makeDecisionLogic 核价决策逻辑
func (d *DefaultPricingDecisionMaker) makeDecisionLogic(pricingCtx *model.PricingContext, result *model.PricingResult) {
	suggestPrice := pricingCtx.SuggestPrice
	acceptablePrice := result.AcceptablePrice

	// 如果建议价格满足要求，接受
	if suggestPrice >= acceptablePrice {
		result.SetDecision(model.ActionAccept,
			fmt.Sprintf("建议价格%.2f满足最低要求%.2f，利润率%.2f%%",
				suggestPrice, acceptablePrice, result.ProfitMargin))
		return
	}

	// 建议价格不满足要求，根据店铺配置决定策略
	strategy := pricingCtx.GetPriceRejectStrategy()

	switch strategy {
	case "TAKE_OFFLINE":
		// 下架策略
		result.SetDecision(model.ActionReject,
			fmt.Sprintf("建议价格%.2f低于最低要求%.2f，根据店铺配置执行下架",
				suggestPrice, acceptablePrice))

	case "KEEP_ONLINE":
		// 保留在售策略
		if pricingCtx.IsRebargainEnabled() {
			result.SetDecision(model.ActionReappeal,
				fmt.Sprintf("建议价格%.2f低于最低要求%.2f，根据店铺配置保留在售并重新议价",
					suggestPrice, acceptablePrice))
		} else {
			result.SetDecision(model.ActionSkip,
				fmt.Sprintf("建议价格%.2f低于最低要求%.2f，店铺未启用重新议价，保留在售",
					suggestPrice, acceptablePrice))
		}

	default:
		// 默认跳过
		result.SetDecision(model.ActionSkip,
			fmt.Sprintf("建议价格%.2f低于最低要求%.2f，未知的拒绝策略: %s",
				suggestPrice, acceptablePrice, strategy))
	}
}

// ValidateContext 验证核价上下文
func (d *DefaultPricingDecisionMaker) ValidateContext(pricingCtx *model.PricingContext) error {
	if pricingCtx == nil {
		return fmt.Errorf("核价上下文不能为空")
	}

	if err := pricingCtx.Validate(); err != nil {
		return fmt.Errorf("核价上下文验证失败: %w", err)
	}

	if len(pricingCtx.PricingRules) == 0 {
		d.logger.Warn("未找到核价规则")
	}

	return nil
}
