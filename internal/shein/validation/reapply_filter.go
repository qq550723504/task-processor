package validation

import (
	"task-processor/internal/core/logger"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	shein "task-processor/internal/shein"
)

type ReapplyFilterRuleHandler struct {
	ruleChecker *FilterRuleChecker
}

func NewReapplyFilterRuleHandler() *ReapplyFilterRuleHandler {
	return &ReapplyFilterRuleHandler{ruleChecker: NewFilterRuleChecker()}
}

func (h *ReapplyFilterRuleHandler) Name() string {
	return "reapply_filter_rule_to_variants"
}

func (h *ReapplyFilterRuleHandler) Handle(ctx *shein.TaskContext) error {
	variants := ctx.FilteredVariants()
	if variants == nil {
		variants = []model.Product{}
		ctx.SetVariants(variants)
	}

	filteredVariants := make([]model.Product, 0, len(variants))
	unfilteredVariants := make([]model.Product, 0, len(variants))

	for _, variant := range variants {
		if err := h.applyFilterRuleToVariant(ctx.FilterRule, variant, ctx); err != nil {
			logger.GetGlobalLogger("shein/validation").Infof("variant filtered out: asin=%s err=%v", variant.Asin, err)
			ctx.SetVariantFiltered(variant.Asin, true, err.Error())
			unfilteredVariants = append(unfilteredVariants, variant)
		} else {
			ctx.SetVariantFiltered(variant.Asin, false, "")
		}
		filteredVariants = append(filteredVariants, variant)
	}

	ctx.SetVariants(filteredVariants)
	ctx.SetUnfilteredVariants(unfilteredVariants)
	logger.GetGlobalLogger("shein/validation").Infof("reapplied filter rule to variants: kept=%d removed=%d", len(filteredVariants), len(unfilteredVariants))
	return nil
}

func (h *ReapplyFilterRuleHandler) applyFilterRuleToVariant(filterRuleDTO *managementapi.FilterRuleRespDTO, variant model.Product, ctx *shein.TaskContext) error {
	priceType := "special"
	if ctx.StoreInfo != nil && ctx.StoreInfo.PriceType != "" {
		priceType = ctx.StoreInfo.PriceType
	}

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
