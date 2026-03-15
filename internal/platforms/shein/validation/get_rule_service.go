package validation

import (
	"fmt"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/platforms/shein"
)

// 验证筛选规则，应用利润规则
type TaskValidatorHandler struct {
	managementClientMgr *management.ClientManager
}

func NewTaskValidatorHandler(managementClientMgr *management.ClientManager) *TaskValidatorHandler {
	return &TaskValidatorHandler{
		managementClientMgr: managementClientMgr,
	}
}

func (t *TaskValidatorHandler) Name() string {
	return "验证筛选规则并应用利润规则"
}

func (t *TaskValidatorHandler) Handle(ctx *shein.TaskContext) error {
	// 获取筛选规则
	filterRuleClient := t.managementClientMgr.GetFilterRuleClient()
	filterRuleReq := &api.FilterRuleReqDTO{
		TenantID:   ctx.Task.TenantID,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
	}

	filterRules, err := filterRuleClient.GetFilterRule(filterRuleReq)
	if err != nil {
		return fmt.Errorf("获取筛选规则失败: %w", err)
	}

	// 检查是否有筛选规则
	if filterRules == nil || len(*filterRules) == 0 {
		return fmt.Errorf("未找到筛选规则")
	}

	// 使用第一个筛选规则
	filterRule := &(*filterRules)[0]

	// 保存筛选规则到上下文
	ctx.FilterRule = filterRule

	// 验证筛选规则（这里可以添加具体的验证逻辑）
	if filterRule.Status != 0 {
		return fmt.Errorf("筛选规则未启用: %s", filterRule.Name)
	}

	// 获取利润规则
	profitRuleClient := t.managementClientMgr.GetProfitRuleClient()
	profitRuleReq := &api.ProfitRuleReqDTO{
		TenantID: ctx.Task.TenantID,
		StoreID:  ctx.Task.StoreID,
	}

	profitRule, err := profitRuleClient.GetProfitRule(profitRuleReq)
	if err != nil {
		return fmt.Errorf("获取利润规则失败: %w", err)
	}

	// 保存利润规则到上下文
	ctx.ProfitRule = profitRule

	// 验证利润规则（这里可以添加具体的验证逻辑）
	if profitRule.Status != 0 {
		return fmt.Errorf("利润规则未启用: %s", profitRule.Name)
	}

	return nil
}


