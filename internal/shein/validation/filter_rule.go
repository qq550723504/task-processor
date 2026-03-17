package validation

import "task-processor/internal/shein"

// ApplyFilterRuleHandler 应用筛选规则处理器
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
	inventory := h.ruleChecker.getInventory(ctx.AmazonProduct)
	if err := h.ruleChecker.CheckInventory(filterRule, inventory); err != nil {
		return err
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
