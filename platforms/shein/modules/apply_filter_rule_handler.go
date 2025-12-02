package modules

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

func (h *ApplyFilterRuleHandler) Handle(ctx *TaskContext) error {
	// 校验价格范围
	priceValue := GetProductPrice(ctx.AmazonProduct, ctx.FilterRule.PriceType)
	if err := h.ruleChecker.CheckPriceRange(ctx.FilterRule, priceValue); err != nil {
		return err
	}

	// 校验库存
	inventory := h.ruleChecker.getInventory(ctx.AmazonProduct)
	if err := h.ruleChecker.CheckInventory(ctx.FilterRule, inventory); err != nil {
		return err
	}

	// 校验发货时效
	deliveryTime := h.ruleChecker.getDeliveryTime(ctx.AmazonProduct)
	if err := h.ruleChecker.CheckDeliveryTime(ctx.FilterRule, deliveryTime); err != nil {
		return err
	}

	// 校验评论星级
	if err := h.ruleChecker.CheckRating(ctx.FilterRule, ctx.AmazonProduct.Rating); err != nil {
		return err
	}

	// 校验评论数量
	if err := h.ruleChecker.CheckReviewCount(ctx.FilterRule, ctx.AmazonProduct.ReviewsCount); err != nil {
		return err
	}

	// 校验配送方式
	if err := h.ruleChecker.CheckFulfillmentType(ctx.FilterRule, ctx.AmazonProduct); err != nil {
		return err
	}

	return nil
}
