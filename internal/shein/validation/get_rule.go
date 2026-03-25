package validation

import (
	"fmt"

	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/shein"
)

type TaskValidatorHandler struct {
	managementClientMgr *management.ClientManager
}

func NewTaskValidatorHandler(managementClientMgr *management.ClientManager) *TaskValidatorHandler {
	return &TaskValidatorHandler{managementClientMgr: managementClientMgr}
}

func (t *TaskValidatorHandler) Name() string {
	return "task_validator"
}

func (t *TaskValidatorHandler) Handle(ctx *shein.TaskContext) error {
	filterRule, err := t.loadFilterRule(ctx)
	if err != nil {
		return err
	}
	if filterRule.Status != 0 {
		return fmt.Errorf("filter rule is not enabled: %s", filterRule.Name)
	}

	profitRule, err := t.loadProfitRule(ctx)
	if err != nil {
		return err
	}
	if profitRule.Status != 0 {
		return fmt.Errorf("profit rule is not enabled: %s", profitRule.Name)
	}

	ctx.SetValidationRules(filterRule, profitRule)
	return nil
}

func (t *TaskValidatorHandler) loadFilterRule(ctx *shein.TaskContext) (*managementapi.FilterRuleRespDTO, error) {
	filterRuleClient := t.managementClientMgr.GetFilterRuleClient()
	filterRuleReq := &managementapi.FilterRuleReqDTO{
		TenantID:   ctx.Task.TenantID,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
	}

	filterRules, err := filterRuleClient.GetFilterRule(filterRuleReq)
	if err != nil {
		return nil, fmt.Errorf("get filter rule failed: %w", err)
	}
	if filterRules == nil || len(*filterRules) == 0 {
		return nil, fmt.Errorf("filter rule not found")
	}

	return &(*filterRules)[0], nil
}

func (t *TaskValidatorHandler) loadProfitRule(ctx *shein.TaskContext) (*managementapi.ProfitRuleRespDTO, error) {
	profitRuleClient := t.managementClientMgr.GetProfitRuleClient()
	profitRuleReq := &managementapi.ProfitRuleReqDTO{
		TenantID: ctx.Task.TenantID,
		StoreID:  ctx.Task.StoreID,
	}

	profitRule, err := profitRuleClient.GetProfitRule(profitRuleReq)
	if err != nil {
		return nil, fmt.Errorf("get profit rule failed: %w", err)
	}

	return profitRule, nil
}
