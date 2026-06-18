package validation

import (
	"task-processor/internal/core/logger"
	"task-processor/internal/shein"
)

// ApplyFilterRuleHandler 应用筛选规则处理器
// 作用对象：主产品（ctx.AmazonProduct）
// 注意：此步骤与 ReapplyFilterRuleHandler 不合并，原因如下：
//   - 本步骤在获取变体数据之前执行，用于快速过滤不符合条件的主产品，避免后续无效的变体抓取
//   - ReapplyFilterRuleHandler 在获取变体数据之后执行，作用于每个变体（ctx.Variants）
//   - 两者检查时机和作用对象不同，合并会破坏管道的早期退出优化
type ApplyFilterRuleHandler struct {
	ruleChecker *FilterRuleChecker
}

func NewApplyFilterRuleHandler() *ApplyFilterRuleHandler {
	return &ApplyFilterRuleHandler{
		ruleChecker: NewFilterRuleChecker(),
	}
}

func (h *ApplyFilterRuleHandler) Name() string {
	return "应用筛选规则"
}

func (h *ApplyFilterRuleHandler) Handle(ctx *shein.TaskContext) error {
	if ctx.FilterRule == nil {
		return nil
	}
	filterRule := ctx.FilterRule.ToFilterRule()

	// 校验价格范围
	priceValue := GetProductPrice(ctx.AmazonProduct, ctx.StoreInfo.PriceType)
	if err := h.ruleChecker.CheckPriceRange(filterRule, priceValue); err != nil {
		return err
	}

	// 校验库存
	if !shouldDeferInventoryFilter(ctx) {
		inventory := h.ruleChecker.getInventory(ctx.AmazonProduct)
		if err := h.ruleChecker.CheckInventory(filterRule, inventory); err != nil {
			return err
		}
	}

	// 校验发货时效
	deliveryTime := h.ruleChecker.getDeliveryTime(ctx.AmazonProduct)
	if err := h.ruleChecker.CheckDeliveryTime(filterRule, deliveryTime); err != nil {
		return err
	}

	// 校验评论星级
	if err := h.ruleChecker.CheckRating(filterRule, ctx.AmazonProduct.Rating); err != nil {
		return err
	}

	// 校验评论数量
	if err := h.ruleChecker.CheckReviewCount(filterRule, ctx.AmazonProduct.ReviewsCount); err != nil {
		return err
	}

	// 校验配送方式
	if err := h.ruleChecker.CheckFulfillmentType(filterRule, ctx.AmazonProduct); err != nil {
		return err
	}

	return nil
}

func shouldDeferInventoryFilter(ctx *shein.TaskContext) bool {
	if ctx == nil || ctx.AmazonProduct == nil {
		return false
	}

	if len(ctx.AmazonProduct.Variations) == 0 {
		return false
	}

	logger.GetGlobalLogger("shein/validation").Infof(
		"defer inventory filter to variant stage: asin=%s variation_count=%d",
		ctx.AmazonProduct.Asin,
		len(ctx.AmazonProduct.Variations),
	)
	return true
}
