package listingsubscription

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

type Service struct {
	repo                      Repository
	now                       func() time.Time
	tenantDisplayNameResolver TenantDisplayNameResolver
}

func NewService(repo Repository) (*Service, error) {
	if repo == nil {
		return nil, errors.New("subscription repository is required")
	}
	s := &Service{
		repo:                      repo,
		now:                       time.Now,
		tenantDisplayNameResolver: fallbackTenantDisplayNameResolver{},
	}
	if err := repo.UpsertDefaultModules(context.Background(), DefaultModules()); err != nil {
		return nil, err
	}
	if err := repo.UpsertDefaultPlans(context.Background(), DefaultPlans()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Service) SetTenantDisplayNameResolver(resolver TenantDisplayNameResolver) {
	if s == nil || resolver == nil {
		return
	}
	s.tenantDisplayNameResolver = resolver
}

func DefaultModules() []Module {
	now := time.Now().UTC()
	return []Module{
		{Code: ModuleStoreManagement, Name: "店铺管理", Description: "店铺 CRUD、统计和高级操作", SortOrder: 10, Active: true, CreatedAt: now, UpdatedAt: now},
		{Code: ModuleTaskImport, Name: "任务导入", Description: "导入任务和商品导入映射", SortOrder: 20, Active: true, CreatedAt: now, UpdatedAt: now},
		{Code: ModuleRules, Name: "规则", Description: "筛选、利润、核价规则和敏感词", SortOrder: 30, Active: true, CreatedAt: now, UpdatedAt: now},
		{Code: ModuleOperationStrategy, Name: "运营策略", Description: "运营策略配置和应用", SortOrder: 40, Active: true, CreatedAt: now, UpdatedAt: now},
		{Code: ModuleStudio, Name: "Studio", Description: "设计生成、产品图生成和异步任务", SortOrder: 50, Active: true, CreatedAt: now, UpdatedAt: now},
		{Code: ModuleOSSStorage, Name: "OSS 存储", Description: "对象存储上传容量计费", SortOrder: 60, Active: true, CreatedAt: now, UpdatedAt: now},
	}
}

func DefaultPlans() []PlanBundle {
	now := time.Now().UTC()
	return []PlanBundle{
		{
			Plan: Plan{Code: PlanBasic, Name: "基础版", Description: "店铺、任务导入和基础规则", SortOrder: 10, Active: true, CreatedAt: now, UpdatedAt: now},
			Modules: []PlanModule{
				{PlanCode: PlanBasic, ModuleCode: ModuleStoreManagement, SortOrder: 10},
				{PlanCode: PlanBasic, ModuleCode: ModuleTaskImport, Limits: map[string]int{"import_tasks": 100}, SortOrder: 20},
				{PlanCode: PlanBasic, ModuleCode: ModuleRules, SortOrder: 30},
				{PlanCode: PlanBasic, ModuleCode: ModuleOSSStorage, Limits: map[string]int{"storage_bytes": 1 * 1024 * 1024 * 1024}, SortOrder: 60},
			},
		},
		{
			Plan: Plan{Code: PlanProfessional, Name: "专业版", Description: "包含运营策略、Studio 和 10GB OSS 存储", SortOrder: 20, Active: true, CreatedAt: now, UpdatedAt: now},
			Modules: []PlanModule{
				{PlanCode: PlanProfessional, ModuleCode: ModuleStoreManagement, SortOrder: 10},
				{PlanCode: PlanProfessional, ModuleCode: ModuleTaskImport, Limits: map[string]int{"import_tasks": 1000}, SortOrder: 20},
				{PlanCode: PlanProfessional, ModuleCode: ModuleRules, SortOrder: 30},
				{PlanCode: PlanProfessional, ModuleCode: ModuleOperationStrategy, SortOrder: 40},
				{PlanCode: PlanProfessional, ModuleCode: ModuleStudio, Limits: map[string]int{"design_jobs": 100, "product_image_jobs": 100}, SortOrder: 50},
				{PlanCode: PlanProfessional, ModuleCode: ModuleOSSStorage, Limits: map[string]int{"storage_bytes": 10 * 1024 * 1024 * 1024}, SortOrder: 60},
			},
		},
		{
			Plan: Plan{Code: PlanEnterprise, Name: "企业版", Description: "完整模块和更高额度", SortOrder: 30, Active: true, CreatedAt: now, UpdatedAt: now},
			Modules: []PlanModule{
				{PlanCode: PlanEnterprise, ModuleCode: ModuleStoreManagement, SortOrder: 10},
				{PlanCode: PlanEnterprise, ModuleCode: ModuleTaskImport, Limits: map[string]int{"import_tasks": 10000}, SortOrder: 20},
				{PlanCode: PlanEnterprise, ModuleCode: ModuleRules, SortOrder: 30},
				{PlanCode: PlanEnterprise, ModuleCode: ModuleOperationStrategy, SortOrder: 40},
				{PlanCode: PlanEnterprise, ModuleCode: ModuleStudio, Limits: map[string]int{"design_jobs": 1000, "product_image_jobs": 1000}, SortOrder: 50},
				{PlanCode: PlanEnterprise, ModuleCode: ModuleOSSStorage, Limits: map[string]int{"storage_bytes": 100 * 1024 * 1024 * 1024}, SortOrder: 60},
			},
		},
	}
}

func (s *Service) ListModules(ctx context.Context) ([]Module, error) {
	modules, err := s.repo.ListModules(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(modules, func(i, j int) bool {
		if modules[i].SortOrder == modules[j].SortOrder {
			return modules[i].Code < modules[j].Code
		}
		return modules[i].SortOrder < modules[j].SortOrder
	})
	return modules, nil
}

func (s *Service) ListPlans(ctx context.Context) ([]PlanBundle, error) {
	plans, err := s.repo.ListPlans(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(plans, func(i, j int) bool {
		if plans[i].Plan.SortOrder == plans[j].Plan.SortOrder {
			return plans[i].Plan.Code < plans[j].Plan.Code
		}
		return plans[i].Plan.SortOrder < plans[j].Plan.SortOrder
	})
	for i := range plans {
		sort.Slice(plans[i].Modules, func(left, right int) bool {
			if plans[i].Modules[left].SortOrder == plans[i].Modules[right].SortOrder {
				return plans[i].Modules[left].ModuleCode < plans[i].Modules[right].ModuleCode
			}
			return plans[i].Modules[left].SortOrder < plans[i].Modules[right].SortOrder
		})
	}
	return plans, nil
}

func (s *Service) GetSummary(ctx context.Context, tenantID string) (*Summary, error) {
	tenantID = strings.TrimSpace(tenantID)
	modules, err := s.ListModules(ctx)
	if err != nil {
		return nil, err
	}
	entitlements, err := s.repo.ListEntitlements(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	usage, err := s.repo.ListUsage(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	entitlementByModule := make(map[string]Entitlement, len(entitlements))
	for _, entitlement := range entitlements {
		entitlementByModule[entitlement.ModuleCode] = entitlement
	}
	usageByModule := make(map[string][]UsageCounter)
	for _, counter := range usage {
		usageByModule[counter.ModuleCode] = append(usageByModule[counter.ModuleCode], counter)
	}

	views := make([]EntitlementView, 0, len(modules))
	for _, module := range modules {
		view := EntitlementView{Module: module, Usage: usageByModule[module.Code], Used: map[string]int{}}
		if entitlement, ok := entitlementByModule[module.Code]; ok {
			entitlementCopy := entitlement
			view.Entitlement = &entitlementCopy
			view.Limits = cloneLimits(entitlement.Limits)
			for _, counter := range view.Usage {
				view.Used[counter.Metric] += counter.Used
			}
			view.Allowed, view.Reason = evaluateEntitlement(&entitlementCopy, s.now())
		} else {
			view.Allowed = false
			view.Reason = "not_configured"
		}
		views = append(views, view)
	}
	summary := &Summary{TenantID: tenantID, Modules: modules, Entitlements: views}
	subscription, err := s.repo.GetTenantSubscription(ctx, tenantID)
	if err != nil && !errors.Is(err, ErrEntitlementNotFound) {
		return nil, err
	}
	if subscription != nil {
		summary.Subscription = subscription
		if plan, ok, err := s.getPlan(ctx, subscription.PlanCode); err != nil {
			return nil, err
		} else if ok {
			summary.CurrentPlan = &plan
		}
	}
	return summary, nil
}

func (s *Service) GetTenantSummary(ctx context.Context, tenantID string) (*Summary, error) {
	return s.GetSummary(ctx, tenantID)
}

func (s *Service) ListTenantOverviews(ctx context.Context) ([]TenantOverview, error) {
	items, err := s.repo.ListTenantOverviews(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].TenantDisplayName = s.resolveTenantDisplayName(ctx, items[i].TenantID)
	}
	sort.Slice(items, func(i, j int) bool {
		left := items[i].UpdatedAt
		right := items[j].UpdatedAt
		if left != nil && right != nil && !left.Equal(*right) {
			return left.After(*right)
		}
		if left != nil && right == nil {
			return true
		}
		if left == nil && right != nil {
			return false
		}
		return items[i].TenantID < items[j].TenantID
	})
	return items, nil
}

func (s *Service) resolveTenantDisplayName(ctx context.Context, tenantID string) string {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return ""
	}
	resolver := s.tenantDisplayNameResolver
	if resolver == nil {
		return tenantID
	}
	displayName, err := resolver.ResolveTenantDisplayName(ctx, tenantID)
	if err != nil {
		return tenantID
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return tenantID
	}
	return displayName
}

func (s *Service) UpsertEntitlement(ctx context.Context, tenantID, moduleCode string, input EntitlementInput) (*Entitlement, error) {
	tenantID = strings.TrimSpace(tenantID)
	moduleCode = strings.TrimSpace(moduleCode)
	if tenantID == "" {
		return nil, errors.New("tenant id is required")
	}
	if moduleCode == "" {
		return nil, errors.New("module code is required")
	}
	if !isValidStatus(input.Status) {
		return nil, errors.New("invalid subscription status")
	}
	if !s.moduleExists(ctx, moduleCode) {
		return nil, ErrModuleNotFound
	}
	return s.repo.UpsertEntitlement(ctx, &Entitlement{
		TenantID:   tenantID,
		ModuleCode: moduleCode,
		Status:     input.Status,
		StartsAt:   input.StartsAt,
		ExpiresAt:  input.ExpiresAt,
		Limits:     cloneLimits(input.Limits),
	})
}

func (s *Service) UpsertEntitlementWithAudit(ctx context.Context, tenantID, moduleCode string, input EntitlementInput, actorID, reason string) (*Entitlement, error) {
	entitlement, err := s.UpsertEntitlement(ctx, tenantID, moduleCode, input)
	if err != nil {
		return nil, err
	}
	_ = s.createAudit(ctx, tenantID, moduleCode, "entitlement_upsert", actorID, reason, input)
	return entitlement, nil
}

func (s *Service) ApplyPlan(ctx context.Context, tenantID string, input PlanApplyInput, actorID string) (*TenantSubscription, error) {
	tenantID = strings.TrimSpace(tenantID)
	input.PlanCode = strings.TrimSpace(input.PlanCode)
	if tenantID == "" {
		return nil, errors.New("tenant id is required")
	}
	if input.PlanCode == "" {
		return nil, errors.New("plan code is required")
	}
	if input.Status == "" {
		input.Status = StatusActive
	}
	if !isValidStatus(input.Status) {
		return nil, errors.New("invalid subscription status")
	}
	plan, ok, err := s.getPlan(ctx, input.PlanCode)
	if err != nil {
		return nil, err
	}
	if !ok || !plan.Plan.Active {
		return nil, ErrModuleNotFound
	}
	subscription, err := s.repo.UpsertTenantSubscription(ctx, &TenantSubscription{
		TenantID:  tenantID,
		PlanCode:  input.PlanCode,
		Status:    input.Status,
		StartsAt:  input.StartsAt,
		ExpiresAt: input.ExpiresAt,
	})
	if err != nil {
		return nil, err
	}
	for _, module := range plan.Modules {
		if _, err := s.UpsertEntitlement(ctx, tenantID, module.ModuleCode, EntitlementInput{
			Status:    input.Status,
			StartsAt:  input.StartsAt,
			ExpiresAt: input.ExpiresAt,
			Limits:    module.Limits,
		}); err != nil {
			return nil, err
		}
	}
	_ = s.createAudit(ctx, tenantID, "", "plan_apply", actorID, input.PlanCode, input)
	return subscription, nil
}

func (s *Service) UpsertPlan(ctx context.Context, input PlanInput, actorID string) (*PlanBundle, error) {
	input.Code = strings.TrimSpace(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	if input.Code == "" {
		return nil, errors.New("plan code is required")
	}
	if input.Name == "" {
		return nil, errors.New("plan name is required")
	}
	modules := make([]PlanModule, 0, len(input.Modules))
	for _, moduleInput := range input.Modules {
		moduleCode := strings.TrimSpace(moduleInput.ModuleCode)
		if moduleCode == "" {
			return nil, errors.New("module code is required")
		}
		if !s.moduleExists(ctx, moduleCode) {
			return nil, ErrModuleNotFound
		}
		modules = append(modules, PlanModule{
			PlanCode:   input.Code,
			ModuleCode: moduleCode,
			Limits:     cloneLimits(moduleInput.Limits),
			SortOrder:  moduleInput.SortOrder,
		})
	}
	now := time.Now().UTC()
	bundle, err := s.repo.UpsertPlan(ctx, Plan{
		Code:        input.Code,
		Name:        input.Name,
		Description: strings.TrimSpace(input.Description),
		SortOrder:   input.SortOrder,
		Active:      input.Active,
		UpdatedAt:   now,
	}, modules)
	if err != nil {
		return nil, err
	}
	_ = s.createAudit(ctx, "", "", "plan_upsert", actorID, input.Code, input)
	return bundle, nil
}

func (s *Service) UpsertPlanModule(ctx context.Context, planCode, moduleCode string, input PlanModuleInput, actorID string) (*PlanBundle, error) {
	planCode = strings.TrimSpace(planCode)
	moduleCode = strings.TrimSpace(moduleCode)
	if planCode == "" {
		return nil, errors.New("plan code is required")
	}
	if moduleCode == "" {
		return nil, errors.New("module code is required")
	}
	if _, ok, err := s.getPlan(ctx, planCode); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrModuleNotFound
	}
	if !s.moduleExists(ctx, moduleCode) {
		return nil, ErrModuleNotFound
	}
	bundle, err := s.repo.UpsertPlanModule(ctx, PlanModule{
		PlanCode:   planCode,
		ModuleCode: moduleCode,
		Limits:     cloneLimits(input.Limits),
		SortOrder:  input.SortOrder,
	})
	if err != nil {
		return nil, err
	}
	_ = s.createAudit(ctx, "", moduleCode, "plan_module_upsert", actorID, planCode, input)
	return bundle, nil
}

func (s *Service) DeletePlanModule(ctx context.Context, planCode, moduleCode, actorID string) (*PlanBundle, error) {
	planCode = strings.TrimSpace(planCode)
	moduleCode = strings.TrimSpace(moduleCode)
	if planCode == "" {
		return nil, errors.New("plan code is required")
	}
	if moduleCode == "" {
		return nil, errors.New("module code is required")
	}
	bundle, err := s.repo.DeletePlanModule(ctx, planCode, moduleCode)
	if err != nil {
		return nil, err
	}
	_ = s.createAudit(ctx, "", moduleCode, "plan_module_delete", actorID, planCode, nil)
	return bundle, nil
}

func (s *Service) SetPlanActive(ctx context.Context, planCode string, active bool, actorID string) (*PlanBundle, error) {
	plan, ok, err := s.getPlan(ctx, strings.TrimSpace(planCode))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrModuleNotFound
	}
	input := PlanInput{
		Code:        plan.Plan.Code,
		Name:        plan.Plan.Name,
		Description: plan.Plan.Description,
		SortOrder:   plan.Plan.SortOrder,
		Active:      active,
		Modules:     make([]PlanModuleInput, 0, len(plan.Modules)),
	}
	for _, module := range plan.Modules {
		input.Modules = append(input.Modules, PlanModuleInput{
			ModuleCode: module.ModuleCode,
			Limits:     module.Limits,
			SortOrder:  module.SortOrder,
		})
	}
	bundle, err := s.UpsertPlan(ctx, input, actorID)
	if err != nil {
		return nil, err
	}
	_ = s.createAudit(ctx, "", "", "plan_status_update", actorID, planCode, map[string]bool{"active": active})
	return bundle, nil
}

func (s *Service) ListPlanTenants(ctx context.Context, planCode string) ([]TenantSubscription, error) {
	planCode = strings.TrimSpace(planCode)
	if planCode == "" {
		return nil, errors.New("plan code is required")
	}
	return s.repo.ListTenantSubscriptionsByPlan(ctx, planCode)
}

func (s *Service) ListPlanAuditLogs(ctx context.Context, planCode string, limit int) ([]AuditLog, error) {
	planCode = strings.TrimSpace(planCode)
	if planCode == "" {
		return nil, errors.New("plan code is required")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repo.ListPlanAuditLogs(ctx, planCode, limit)
}

func (s *Service) SetUsage(ctx context.Context, tenantID, moduleCode string, input UsageAdjustmentInput, actorID string) (*UsageCounter, error) {
	tenantID = strings.TrimSpace(tenantID)
	moduleCode = strings.TrimSpace(moduleCode)
	input.PeriodKey = strings.TrimSpace(input.PeriodKey)
	input.Metric = strings.TrimSpace(input.Metric)
	if tenantID == "" {
		return nil, errors.New("tenant id is required")
	}
	if moduleCode == "" {
		return nil, errors.New("module code is required")
	}
	if input.PeriodKey == "" {
		input.PeriodKey = s.now().UTC().Format("2006-01")
	}
	if input.Metric == "" {
		return nil, errors.New("usage metric is required")
	}
	if input.Used < 0 {
		return nil, errors.New("usage used must be non-negative")
	}
	if !s.moduleExists(ctx, moduleCode) {
		return nil, ErrModuleNotFound
	}
	counter, err := s.repo.SetUsage(ctx, tenantID, moduleCode, input.PeriodKey, input.Metric, input.Used)
	if err != nil {
		return nil, err
	}
	_ = s.createAudit(ctx, tenantID, moduleCode, "usage_set", actorID, input.Reason, input)
	return counter, nil
}

func (s *Service) ListAuditLogs(ctx context.Context, tenantID string, limit int) ([]AuditLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repo.ListAuditLogs(ctx, strings.TrimSpace(tenantID), limit)
}

func (s *Service) Check(ctx context.Context, tenantID, moduleCode string) (GuardResult, error) {
	return s.CheckUsage(ctx, tenantID, moduleCode, "", 0)
}

func (s *Service) CheckUsage(ctx context.Context, tenantID, moduleCode, metric string, increment int) (GuardResult, error) {
	result, err := s.AuthorizeUsage(ctx, tenantID, moduleCode, metric, increment)
	if err != nil {
		return result, err
	}
	if metric == "" || increment <= 0 {
		return result, nil
	}
	counter, err := s.RecordUsage(ctx, tenantID, moduleCode, metric, increment)
	if err != nil {
		return result, err
	}
	result.Used = counter.Used
	return result, nil
}

func (s *Service) AuthorizeUsage(ctx context.Context, tenantID, moduleCode, metric string, increment int) (GuardResult, error) {
	tenantID = strings.TrimSpace(tenantID)
	moduleCode = strings.TrimSpace(moduleCode)
	result := GuardResult{ModuleCode: moduleCode, Metric: metric}
	entitlement, err := s.repo.GetEntitlement(ctx, tenantID, moduleCode)
	if err != nil {
		if errors.Is(err, ErrEntitlementNotFound) {
			result.Reason = "not_configured"
			return result, ErrSubscriptionRequired
		}
		return result, err
	}
	allowed, reason := evaluateEntitlement(entitlement, s.now())
	if !allowed {
		result.Reason = reason
		return result, ErrSubscriptionRequired
	}
	if metric == "" || increment <= 0 {
		result.Allowed = true
		return result, nil
	}
	limit := entitlement.Limits[metric]
	currentUsed, err := s.currentPeriodUsage(ctx, tenantID, moduleCode, metric)
	if err != nil {
		return result, err
	}
	result.Limit = limit
	result.Used = currentUsed + increment
	if limit > 0 && result.Used > limit {
		result.Reason = "quota_exceeded"
		return result, ErrSubscriptionQuotaExceed
	}
	result.Allowed = true
	return result, nil
}

func (s *Service) RecordUsage(ctx context.Context, tenantID, moduleCode, metric string, increment int) (*UsageCounter, error) {
	tenantID = strings.TrimSpace(tenantID)
	moduleCode = strings.TrimSpace(moduleCode)
	metric = strings.TrimSpace(metric)
	if tenantID == "" {
		return nil, errors.New("tenant id is required")
	}
	if moduleCode == "" {
		return nil, errors.New("module code is required")
	}
	if metric == "" {
		return nil, errors.New("usage metric is required")
	}
	if increment == 0 {
		return nil, errors.New("usage increment cannot be zero")
	}
	if !s.moduleExists(ctx, moduleCode) {
		return nil, ErrModuleNotFound
	}
	if increment < 0 {
		currentUsed, err := s.currentPeriodUsage(ctx, tenantID, moduleCode, metric)
		if err != nil {
			return nil, err
		}
		if currentUsed+increment < 0 {
			increment = -currentUsed
		}
	}
	periodKey := s.now().UTC().Format("2006-01")
	return s.repo.IncrementUsage(ctx, tenantID, moduleCode, periodKey, metric, increment)
}

func (s *Service) currentPeriodUsage(ctx context.Context, tenantID, moduleCode, metric string) (int, error) {
	periodKey := s.now().UTC().Format("2006-01")
	usage, err := s.repo.ListUsage(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	used := 0
	for _, counter := range usage {
		if counter.ModuleCode == moduleCode && counter.PeriodKey == periodKey && counter.Metric == metric {
			used += counter.Used
		}
	}
	return used, nil
}

func (s *Service) moduleExists(ctx context.Context, moduleCode string) bool {
	modules, err := s.ListModules(ctx)
	if err != nil {
		return false
	}
	for _, module := range modules {
		if module.Code == moduleCode {
			return true
		}
	}
	return false
}

func (s *Service) getPlan(ctx context.Context, planCode string) (PlanBundle, bool, error) {
	plans, err := s.ListPlans(ctx)
	if err != nil {
		return PlanBundle{}, false, err
	}
	for _, plan := range plans {
		if plan.Plan.Code == planCode {
			return plan, true, nil
		}
	}
	return PlanBundle{}, false, nil
}

func (s *Service) createAudit(ctx context.Context, tenantID, moduleCode, action, actorID, reason string, payload any) error {
	data, _ := json.Marshal(payload)
	_, err := s.repo.CreateAuditLog(ctx, AuditLog{
		TenantID:   strings.TrimSpace(tenantID),
		ModuleCode: strings.TrimSpace(moduleCode),
		Action:     action,
		ActorID:    strings.TrimSpace(actorID),
		Reason:     strings.TrimSpace(reason),
		Payload:    string(data),
		CreatedAt:  s.now().UTC(),
	})
	return err
}

func evaluateEntitlement(entitlement *Entitlement, now time.Time) (bool, string) {
	if entitlement == nil {
		return false, "not_configured"
	}
	switch entitlement.Status {
	case StatusActive, StatusTrialing:
	default:
		return false, entitlement.Status
	}
	if entitlement.StartsAt != nil && now.Before(*entitlement.StartsAt) {
		return false, "not_started"
	}
	if entitlement.ExpiresAt != nil && !now.Before(*entitlement.ExpiresAt) {
		return false, StatusExpired
	}
	return true, ""
}

func isValidStatus(status string) bool {
	switch status {
	case StatusActive, StatusTrialing, StatusExpired, StatusDisabled:
		return true
	default:
		return false
	}
}

func cloneLimits(in map[string]int) map[string]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int, len(in))
	for key, value := range in {
		if trimmed := strings.TrimSpace(key); trimmed != "" && value >= 0 {
			out[trimmed] = value
		}
	}
	return out
}
