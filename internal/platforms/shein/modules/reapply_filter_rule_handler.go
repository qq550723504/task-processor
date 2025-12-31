package modules

import (
	"task-processor/internal/common/management/api"
	"task-processor/internal/model"

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
func (h *ReapplyFilterRuleHandler) Handle(ctx *TaskContext) error {
	// 检查ctx.Variants是否为nil，如果是则初始化
	if ctx.Variants == nil {
		variants := make([]model.Product, 0)
		ctx.Variants = &variants
	}

	// 对每个变体应用筛选规则
	filteredVariants := make([]model.Product, 0, len(*ctx.Variants))
	unFilteredVariants := make([]model.Product, 0, len(*ctx.Variants))

	for _, variant := range *ctx.Variants {
		// 对变体应用筛选规则
		if err := h.applyFilterRuleToVariant(ctx.FilterRule, variant, ctx); err != nil {
			logrus.Infof("变体ASIN %s 不符合筛选规则: %v\n", variant.Asin, err)
			// 标记该变体不符合规则
			ctx.SetVariantFiltered(variant.Asin, true, err.Error())
			unFilteredVariants = append(unFilteredVariants, variant)
		} else {
			// 符合规则的变体
			ctx.SetVariantFiltered(variant.Asin, false, "")
		}
		filteredVariants = append(filteredVariants, variant)
	}

	// 更新上下文中的变体数据
	*ctx.Variants = filteredVariants

	// 检查ctx.UnFilteredVariants是否为nil，如果是则初始化
	if ctx.UnFilteredVariants == nil {
		unFilteredVariantsPtr := make([]model.Product, 0)
		ctx.UnFilteredVariants = &unFilteredVariantsPtr
	}
	*ctx.UnFilteredVariants = unFilteredVariants

	logrus.Infof("完成对 %d 个变体的筛选规则重新应用\n", len(filteredVariants))
	return nil
}

// applyFilterRuleToVariant 对单个变体应用筛选规则
func (h *ReapplyFilterRuleHandler) applyFilterRuleToVariant(filterRule *api.FilterRuleRespDTO, variant model.Product, ctx *TaskContext) error {
	// 获取店铺配置的价格类型
	priceType := "special"
	if ctx.StoreInfo != nil && ctx.StoreInfo.PriceType != "" {
		priceType = ctx.StoreInfo.PriceType
	}

	// 获取产品价格
	priceValue := GetProductPrice(&variant, priceType)

	// 校验价格范围
	if err := h.ruleChecker.CheckPriceRange(filterRule, priceValue); err != nil {
		return err
	}

	// 校验库存
	inventory := h.ruleChecker.getInventory(&variant)
	if err := h.ruleChecker.CheckInventory(filterRule, inventory); err != nil {
		return err
	}

	// 校验发货时效
	deliveryTime := h.ruleChecker.getDeliveryTime(&variant)
	if err := h.ruleChecker.CheckDeliveryTime(filterRule, deliveryTime); err != nil {
		return err
	}

	// 校验评论星级
	if err := h.ruleChecker.CheckRating(filterRule, variant.Rating); err != nil {
		return err
	}

	// 校验评论数量
	if err := h.ruleChecker.CheckReviewCount(filterRule, variant.ReviewsCount); err != nil {
		return err
	}

	// 校验配送方式
	if err := h.ruleChecker.CheckFulfillmentType(filterRule, &variant); err != nil {
		return err
	}

	return nil
}
