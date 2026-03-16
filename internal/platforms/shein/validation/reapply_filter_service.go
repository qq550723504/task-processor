package validation

import (
	"task-processor/internal/domain/model"
	"task-processor/internal/infra/clients/management/api"
	shein "task-processor/internal/platforms/shein"

	"github.com/sirupsen/logrus"
)

// ReapplyFilterRuleHandler 重新应用筛选规则处理器
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
			logrus.Infof("变体ASIN %s 不符合筛选规则: %v\n", variant.Asin, err)
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

	logrus.Infof("完成对 %d 个变体的筛选规则重新应用\n", len(filteredVariants))
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
