package validation

import (
	"task-processor/internal/core/logger"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	shein "task-processor/internal/shein"
)

// ReapplyFilterRuleHandler 重新应用筛选规则处理器
// 作用对象：变体列表（ctx.Variants）
// 注意：此步骤与 ApplyFilterRuleHandler 不合并，原因如下：
//   - 本步骤在获取变体数据之后执行，对每个变体单独应用筛选规则，并记录过滤状态
//   - ApplyFilterRuleHandler 在获取变体数据之前执行，作用于主产品，用于早期退出
//   - 两者检查时机和作用对象不同，合并会破坏管道的早期退出优化
type ReapplyFilterRuleHandler struct {
	ruleChecker *FilterRuleChecker
}

// NewReapplyFilterRuleHandler 创建新的重新应用筛选规则处理器
func NewReapplyFilterRuleHandler() *ReapplyFilterRuleHandler {
	return &ReapplyFilterRuleHandler{
		ruleChecker: NewFilterRuleChecker(),
	}
}

// Name 返回处理器名称
func (h *ReapplyFilterRuleHandler) Name() string {
	return "重新应用筛选规则到变体"
}

// Handle 执行重新应用筛选规则处理
func (h *ReapplyFilterRuleHandler) Handle(ctx *shein.TaskContext) error {
	if ctx.Variants == nil {
		variants := make([]model.Product, 0)
		ctx.Variants = &variants
	}

	filteredVariants := make([]model.Product, 0, len(*ctx.Variants))
	unFilteredVariants := make([]model.Product, 0, len(*ctx.Variants))

	for _, variant := range *ctx.Variants {
		if err := h.applyFilterRuleToVariant(ctx.FilterRule, variant, ctx); err != nil {
			logger.GetGlobalLogger("shein/validation").Infof("变体ASIN %s 不符合筛选规则: %v\n", variant.Asin, err)
			ctx.SetVariantFiltered(variant.Asin, true, err.Error())
			unFilteredVariants = append(unFilteredVariants, variant)
		} else {
			ctx.SetVariantFiltered(variant.Asin, false, "")
		}
		filteredVariants = append(filteredVariants, variant)
	}

	*ctx.Variants = filteredVariants

	if ctx.UnFilteredVariants == nil {
		unFilteredVariantsPtr := make([]model.Product, 0)
		ctx.UnFilteredVariants = &unFilteredVariantsPtr
	}
	*ctx.UnFilteredVariants = unFilteredVariants

	logger.GetGlobalLogger("shein/validation").Infof("完成对 %d 个变体的筛选规则重新应用\n", len(filteredVariants))
	return nil
}

// applyFilterRuleToVariant 对单个变体应用筛选规则
func (h *ReapplyFilterRuleHandler) applyFilterRuleToVariant(filterRuleDTO *api.FilterRuleRespDTO, variant model.Product, ctx *shein.TaskContext) error {
	priceType := "special"
	if ctx.StoreInfo != nil && ctx.StoreInfo.PriceType != "" {
		priceType = ctx.StoreInfo.PriceType
	}

	// 转换为 domain FilterRule
	filterRule := filterRuleDTO.ToFilterRule()

	priceValue := GetProductPrice(&variant, priceType)

	if err := h.ruleChecker.CheckPriceRange(filterRule, priceValue); err != nil {
		return err
	}

	inventory := h.ruleChecker.getInventory(&variant)
	if err := h.ruleChecker.CheckInventory(filterRule, inventory); err != nil {
		return err
	}

	deliveryTime := h.ruleChecker.getDeliveryTime(&variant)
	if err := h.ruleChecker.CheckDeliveryTime(filterRule, deliveryTime); err != nil {
		return err
	}

	if err := h.ruleChecker.CheckRating(filterRule, variant.Rating); err != nil {
		return err
	}

	if err := h.ruleChecker.CheckReviewCount(filterRule, variant.ReviewsCount); err != nil {
		return err
	}

	if err := h.ruleChecker.CheckFulfillmentType(filterRule, &variant); err != nil {
		return err
	}

	return nil
}
