// Package filter 提供TEMU平台的各种处理器，包括筛选规则处理等功能
package filter

import (
	"fmt"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/temu/context"
	"task-processor/internal/temu/product"
	"task-processor/internal/temu/rules"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// FilterRuleHandler 筛选规则处理器
type FilterRuleHandler struct {
	logger           *logrus.Entry
	filterRuleClient api.FilterRuleAPI

	// 注入的专职处理器
	ruleManager    *FilterRuleManager
	productChecker *product.ProductFilterChecker
	ruleValidator  *rules.RuleValidator
	statsProvider  *FilterRuleStatsProvider
}

// NewFilterRuleHandler 创建新的筛选规则处理器
func NewFilterRuleHandler(filterRuleClient api.FilterRuleAPI) *FilterRuleHandler {
	logger := logger.GetGlobalLogger("FilterRuleHandler")

	// 创建专职处理器
	ruleManager := NewFilterRuleManager(filterRuleClient, logger)
	productChecker := product.NewProductFilterChecker(logger)
	ruleValidator := rules.NewRuleValidator(logger)
	statsProvider := NewFilterRuleStatsProvider(ruleManager, logger)

	return &FilterRuleHandler{
		logger:           logger,
		filterRuleClient: filterRuleClient,
		ruleManager:      ruleManager,
		productChecker:   productChecker,
		ruleValidator:    ruleValidator,
		statsProvider:    statsProvider,
	}
}

// Name 返回处理器名称
func (h *FilterRuleHandler) Name() string {
	return "筛选规则处理器"
}

// Handle 处理任务（兼容pipeline.Handler接口）
func (h *FilterRuleHandler) Handle(ctx pipeline.TaskContext) error {
	// 类型断言为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return h.HandleTemu(temuCtx)
}

// HandleTemu 处理任务 - 在任务开始时筛选主产品（强类型上下文）
func (h *FilterRuleHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始应用筛选规则 - 主产品筛选")

	// 获取Amazon产品数据
	amazonProduct := temuCtx.AmazonProduct
	if amazonProduct == nil {
		return fmt.Errorf("Amazon产品数据为空")
	}

	// 获取任务信息
	task := temuCtx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	// 获取筛选规则
	rules, err := h.ruleManager.GetFilterRules(temuCtx.DefaultTaskContext)
	if err != nil {
		// 详细记录错误信息，但不阻断流程
		h.logger.WithFields(logrus.Fields{
			"tenant_id":   task.TenantID,
			"store_id":    task.StoreID,
			"category_id": task.CategoryID,
			"error":       err.Error(),
		}).Warn("获取筛选规则失败，跳过筛选")
		return nil // 不阻断流程，继续执行
	}

	// 检查是否有启用的规则
	if rules == nil || len(*rules) == 0 {
		h.logger.Info("未配置筛选规则，跳过筛选")
		return nil
	}

	// 保存第一个启用的规则到强类型上下文，供后续保存使用
	for _, rule := range *rules {
		if rule.Status == 0 { // 0表示启用
			temuCtx.FilterRule = &rule
			h.logger.Infof("保存筛选规则到context: ID=%d, Name=%s", rule.ID, rule.Name)
			break
		}
	}

	// 应用筛选规则到主产品
	checkResult := h.productChecker.CheckProductAgainstRulesDetailed(amazonProduct, rules, temuCtx.DefaultTaskContext)
	if !checkResult.Passed {
		// 详细记录筛选失败的原因
		h.logger.WithFields(logrus.Fields{
			"asin":           amazonProduct.Asin,
			"title":          amazonProduct.Title,
			"failed_rule":    checkResult.FailedRule,
			"failure_reason": checkResult.FailureReason,
			"product_value":  checkResult.ProductValue,
			"rule_value":     checkResult.RuleValue,
		}).Warn("主产品不符合筛选规则，任务终止")

		return fmt.Errorf("TERMINATED: 主产品不符合筛选规则 - %s (规则: %s)",
			checkResult.FailureReason, checkResult.FailedRule)
	}

	h.logger.Infof("主产品通过筛选规则检查: %s", amazonProduct.Asin)
	return nil
}

// FilterVariants 筛选变体产品（在获取变体后调用）
func (h *FilterRuleHandler) FilterVariants(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始应用筛选规则 - 变体筛选")

	// 获取变体数据
	variants := temuCtx.Variants
	if len(variants) == 0 {
		h.logger.Info("没有变体需要筛选")
		return nil
	}

	// 获取任务信息
	task := temuCtx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	// 获取筛选规则
	rules, err := h.ruleManager.GetFilterRules(temuCtx.DefaultTaskContext)
	if err != nil {
		// 详细记录错误信息，但不阻断流程
		h.logger.WithFields(logrus.Fields{
			"tenant_id":   task.TenantID,
			"store_id":    task.StoreID,
			"category_id": task.CategoryID,
			"error":       err.Error(),
		}).Warn("获取筛选规则失败，跳过变体筛选")
		return nil
	}

	// 检查是否有启用的规则
	if rules == nil || len(*rules) == 0 {
		h.logger.Info("未配置筛选规则，跳过变体筛选")
		return nil
	}

	// 筛选变体
	var filteredVariants []*model.Product
	originalCount := len(variants)

	for _, variant := range variants {
		checkResult := h.productChecker.CheckProductAgainstRulesDetailed(variant, rules, temuCtx.DefaultTaskContext)
		if checkResult.Passed {
			filteredVariants = append(filteredVariants, variant)
			h.logger.Debugf("变体通过筛选: %s", variant.Asin)
		} else {
			h.logger.WithFields(logrus.Fields{
				"asin":           variant.Asin,
				"failed_rule":    checkResult.FailedRule,
				"failure_reason": checkResult.FailureReason,
				"product_value":  checkResult.ProductValue,
				"rule_value":     checkResult.RuleValue,
			}).Info("变体被筛选规则过滤")
		}
	}

	// 更新变体列表
	temuCtx.Variants = filteredVariants
	filteredCount := len(filteredVariants)

	h.logger.Infof("变体筛选完成: 原始数量=%d, 筛选后数量=%d, 过滤数量=%d",
		originalCount, filteredCount, originalCount-filteredCount)

	// 如果所有变体都被过滤掉，记录警告但不阻断流程
	if filteredCount == 0 {
		h.logger.Warn("所有变体都被筛选规则过滤，将只处理主产品")
	}

	return nil
}

// GetFilterRuleStats 获取筛选规则统计信息（用于调试和监控）
func (h *FilterRuleHandler) GetFilterRuleStats(temuCtx *temucontext.TemuTaskContext) (map[string]any, error) {
	return h.statsProvider.GetFilterRuleStats(temuCtx.DefaultTaskContext)
}
