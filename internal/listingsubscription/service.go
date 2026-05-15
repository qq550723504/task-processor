package listingsubscription

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
)

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) (*Service, error) {
	if repo == nil {
		return nil, errors.New("subscription repository is required")
	}
	s := &Service{repo: repo, now: time.Now}
	if err := repo.UpsertDefaultModules(context.Background(), DefaultModules()); err != nil {
		return nil, err
	}
	return s, nil
}

func DefaultModules() []Module {
	now := time.Now().UTC()
	return []Module{
		{Code: ModuleStoreManagement, Name: "店铺管理", Description: "店铺 CRUD、统计和高级操作", SortOrder: 10, Active: true, CreatedAt: now, UpdatedAt: now},
		{Code: ModuleTaskImport, Name: "任务导入", Description: "导入任务和商品导入映射", SortOrder: 20, Active: true, CreatedAt: now, UpdatedAt: now},
		{Code: ModuleRules, Name: "规则", Description: "筛选、利润、核价规则和敏感词", SortOrder: 30, Active: true, CreatedAt: now, UpdatedAt: now},
		{Code: ModuleOperationStrategy, Name: "运营策略", Description: "运营策略配置和应用", SortOrder: 40, Active: true, CreatedAt: now, UpdatedAt: now},
		{Code: ModuleStudio, Name: "Studio", Description: "设计生成、产品图生成和异步任务", SortOrder: 50, Active: true, CreatedAt: now, UpdatedAt: now},
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
	return &Summary{TenantID: tenantID, Modules: modules, Entitlements: views}, nil
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

func (s *Service) Check(ctx context.Context, tenantID, moduleCode string) (GuardResult, error) {
	return s.CheckUsage(ctx, tenantID, moduleCode, "", 0)
}

func (s *Service) CheckUsage(ctx context.Context, tenantID, moduleCode, metric string, increment int) (GuardResult, error) {
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
	if limit <= 0 {
		result.Allowed = true
		return result, nil
	}
	periodKey := s.now().UTC().Format("2006-01")
	counter, err := s.repo.IncrementUsage(ctx, tenantID, moduleCode, periodKey, metric, increment)
	if err != nil {
		return result, err
	}
	result.Limit = limit
	result.Used = counter.Used
	if counter.Used > limit {
		result.Reason = "quota_exceeded"
		return result, ErrSubscriptionQuotaExceed
	}
	result.Allowed = true
	return result, nil
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
