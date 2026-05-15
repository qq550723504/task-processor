package listingsubscription

import (
	"context"
	"sync"
	"time"
)

type MemRepository struct {
	mu            sync.Mutex
	nextID        int64
	modules       map[string]Module
	plans         map[string]PlanBundle
	subscriptions map[string]TenantSubscription
	entitlements  map[string]Entitlement
	usage         map[string]UsageCounter
	auditLogs     []AuditLog
}

func NewMemRepository() *MemRepository {
	return &MemRepository{
		nextID:        1,
		modules:       map[string]Module{},
		plans:         map[string]PlanBundle{},
		subscriptions: map[string]TenantSubscription{},
		entitlements:  map[string]Entitlement{},
		usage:         map[string]UsageCounter{},
		auditLogs:     []AuditLog{},
	}
}

func (r *MemRepository) ListModules(context.Context) ([]Module, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]Module, 0, len(r.modules))
	for _, module := range r.modules {
		items = append(items, module)
	}
	return items, nil
}

func (r *MemRepository) UpsertDefaultModules(_ context.Context, modules []Module) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	for _, module := range modules {
		if existing, ok := r.modules[module.Code]; ok {
			existing.Name = module.Name
			existing.Description = module.Description
			existing.SortOrder = module.SortOrder
			existing.Active = module.Active
			existing.UpdatedAt = now
			r.modules[module.Code] = existing
			continue
		}
		if module.CreatedAt.IsZero() {
			module.CreatedAt = now
		}
		module.UpdatedAt = now
		r.modules[module.Code] = module
	}
	return nil
}

func (r *MemRepository) ListPlans(context.Context) ([]PlanBundle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]PlanBundle, 0, len(r.plans))
	for _, plan := range r.plans {
		items = append(items, clonePlanBundle(plan))
	}
	return items, nil
}

func (r *MemRepository) UpsertDefaultPlans(_ context.Context, plans []PlanBundle) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	for _, bundle := range plans {
		existing, ok := r.plans[bundle.Plan.Code]
		if ok {
			existing.Plan.Name = bundle.Plan.Name
			existing.Plan.Description = bundle.Plan.Description
			existing.Plan.SortOrder = bundle.Plan.SortOrder
			existing.Plan.Active = bundle.Plan.Active
			existing.Plan.UpdatedAt = now
			existing.Modules = clonePlanModules(bundle.Modules)
			r.plans[bundle.Plan.Code] = existing
			continue
		}
		if bundle.Plan.CreatedAt.IsZero() {
			bundle.Plan.CreatedAt = now
		}
		bundle.Plan.UpdatedAt = now
		bundle.Modules = clonePlanModules(bundle.Modules)
		r.plans[bundle.Plan.Code] = bundle
	}
	return nil
}

func (r *MemRepository) UpsertPlan(_ context.Context, plan Plan, modules []PlanModule) (*PlanBundle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	existing, ok := r.plans[plan.Code]
	if ok {
		plan.CreatedAt = existing.Plan.CreatedAt
	} else {
		plan.CreatedAt = now
	}
	plan.UpdatedAt = now
	for i := range modules {
		modules[i].PlanCode = plan.Code
		modules[i].Limits = cloneLimits(modules[i].Limits)
	}
	bundle := PlanBundle{Plan: plan, Modules: clonePlanModules(modules)}
	r.plans[plan.Code] = bundle
	return clonePlanBundlePtr(bundle), nil
}

func (r *MemRepository) UpsertPlanModule(_ context.Context, module PlanModule) (*PlanBundle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	bundle, ok := r.plans[module.PlanCode]
	if !ok {
		return nil, ErrModuleNotFound
	}
	module.Limits = cloneLimits(module.Limits)
	replaced := false
	for i := range bundle.Modules {
		if bundle.Modules[i].ModuleCode == module.ModuleCode {
			bundle.Modules[i] = module
			replaced = true
			break
		}
	}
	if !replaced {
		bundle.Modules = append(bundle.Modules, module)
	}
	bundle.Plan.UpdatedAt = time.Now().UTC()
	r.plans[module.PlanCode] = bundle
	return clonePlanBundlePtr(bundle), nil
}

func (r *MemRepository) DeletePlanModule(_ context.Context, planCode, moduleCode string) (*PlanBundle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	bundle, ok := r.plans[planCode]
	if !ok {
		return nil, ErrModuleNotFound
	}
	modules := make([]PlanModule, 0, len(bundle.Modules))
	for _, module := range bundle.Modules {
		if module.ModuleCode != moduleCode {
			modules = append(modules, module)
		}
	}
	bundle.Modules = modules
	bundle.Plan.UpdatedAt = time.Now().UTC()
	r.plans[planCode] = bundle
	return clonePlanBundlePtr(bundle), nil
}

func (r *MemRepository) GetTenantSubscription(_ context.Context, tenantID string) (*TenantSubscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	subscription, ok := r.subscriptions[tenantID]
	if !ok {
		return nil, ErrEntitlementNotFound
	}
	return cloneTenantSubscription(subscription), nil
}

func (r *MemRepository) UpsertTenantSubscription(_ context.Context, subscription *TenantSubscription) (*TenantSubscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	existing, ok := r.subscriptions[subscription.TenantID]
	if !ok {
		existing.ID = r.nextID
		r.nextID++
		existing.CreatedAt = now
	}
	existing.TenantID = subscription.TenantID
	existing.PlanCode = subscription.PlanCode
	existing.Status = subscription.Status
	existing.StartsAt = cloneTime(subscription.StartsAt)
	existing.ExpiresAt = cloneTime(subscription.ExpiresAt)
	existing.UpdatedAt = now
	r.subscriptions[subscription.TenantID] = existing
	return cloneTenantSubscription(existing), nil
}

func (r *MemRepository) GetEntitlement(_ context.Context, tenantID, moduleCode string) (*Entitlement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entitlement, ok := r.entitlements[entitlementKey(tenantID, moduleCode)]
	if !ok {
		return nil, ErrEntitlementNotFound
	}
	return cloneEntitlement(entitlement), nil
}

func (r *MemRepository) ListEntitlements(_ context.Context, tenantID string) ([]Entitlement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := []Entitlement{}
	for _, entitlement := range r.entitlements {
		if entitlement.TenantID == tenantID {
			items = append(items, *cloneEntitlement(entitlement))
		}
	}
	return items, nil
}

func (r *MemRepository) ListTenantOverviews(context.Context) ([]TenantOverview, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	byTenant := map[string]TenantOverview{}
	for _, entitlement := range r.entitlements {
		item := byTenant[entitlement.TenantID]
		item.TenantID = entitlement.TenantID
		item.EntitlementCount++
		if entitlement.Status == StatusActive || entitlement.Status == StatusTrialing {
			item.ActiveCount++
		}
		if item.UpdatedAt == nil || entitlement.UpdatedAt.After(*item.UpdatedAt) {
			updatedAt := entitlement.UpdatedAt
			item.UpdatedAt = &updatedAt
		}
		byTenant[entitlement.TenantID] = item
	}
	items := make([]TenantOverview, 0, len(byTenant))
	for _, item := range byTenant {
		items = append(items, item)
	}
	return items, nil
}

func (r *MemRepository) UpsertEntitlement(_ context.Context, entitlement *Entitlement) (*Entitlement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	key := entitlementKey(entitlement.TenantID, entitlement.ModuleCode)
	existing, ok := r.entitlements[key]
	if !ok {
		existing.ID = r.nextID
		r.nextID++
		existing.CreatedAt = now
	}
	existing.TenantID = entitlement.TenantID
	existing.ModuleCode = entitlement.ModuleCode
	existing.Status = entitlement.Status
	existing.StartsAt = cloneTime(entitlement.StartsAt)
	existing.ExpiresAt = cloneTime(entitlement.ExpiresAt)
	existing.Limits = cloneLimits(entitlement.Limits)
	existing.UpdatedAt = now
	r.entitlements[key] = existing
	return cloneEntitlement(existing), nil
}

func (r *MemRepository) ListUsage(_ context.Context, tenantID string) ([]UsageCounter, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := []UsageCounter{}
	for _, counter := range r.usage {
		if counter.TenantID == tenantID {
			items = append(items, counter)
		}
	}
	return items, nil
}

func (r *MemRepository) IncrementUsage(_ context.Context, tenantID, moduleCode, periodKey, metric string, amount int) (*UsageCounter, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := usageKey(tenantID, moduleCode, periodKey, metric)
	counter := r.usage[key]
	if counter.ID == 0 {
		counter.ID = r.nextID
		r.nextID++
		counter.TenantID = tenantID
		counter.ModuleCode = moduleCode
		counter.PeriodKey = periodKey
		counter.Metric = metric
	}
	counter.Used += amount
	counter.UpdatedAt = time.Now().UTC()
	r.usage[key] = counter
	return &counter, nil
}

func (r *MemRepository) SetUsage(_ context.Context, tenantID, moduleCode, periodKey, metric string, used int) (*UsageCounter, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := usageKey(tenantID, moduleCode, periodKey, metric)
	counter := r.usage[key]
	if counter.ID == 0 {
		counter.ID = r.nextID
		r.nextID++
		counter.TenantID = tenantID
		counter.ModuleCode = moduleCode
		counter.PeriodKey = periodKey
		counter.Metric = metric
	}
	counter.Used = used
	counter.UpdatedAt = time.Now().UTC()
	r.usage[key] = counter
	return &counter, nil
}

func (r *MemRepository) CreateAuditLog(_ context.Context, log AuditLog) (*AuditLog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	log.ID = r.nextID
	r.nextID++
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now().UTC()
	}
	r.auditLogs = append(r.auditLogs, log)
	out := log
	return &out, nil
}

func (r *MemRepository) ListAuditLogs(_ context.Context, tenantID string, limit int) ([]AuditLog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := []AuditLog{}
	for i := len(r.auditLogs) - 1; i >= 0 && len(items) < limit; i-- {
		if r.auditLogs[i].TenantID == tenantID {
			items = append(items, r.auditLogs[i])
		}
	}
	return items, nil
}

func entitlementKey(tenantID, moduleCode string) string {
	return tenantID + "\x00" + moduleCode
}

func usageKey(tenantID, moduleCode, periodKey, metric string) string {
	return tenantID + "\x00" + moduleCode + "\x00" + periodKey + "\x00" + metric
}

func cloneEntitlement(entitlement Entitlement) *Entitlement {
	cloned := entitlement
	cloned.StartsAt = cloneTime(entitlement.StartsAt)
	cloned.ExpiresAt = cloneTime(entitlement.ExpiresAt)
	cloned.Limits = cloneLimits(entitlement.Limits)
	return &cloned
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func clonePlanBundle(bundle PlanBundle) PlanBundle {
	out := bundle
	out.Modules = clonePlanModules(bundle.Modules)
	return out
}

func clonePlanBundlePtr(bundle PlanBundle) *PlanBundle {
	cloned := clonePlanBundle(bundle)
	return &cloned
}

func clonePlanModules(modules []PlanModule) []PlanModule {
	out := make([]PlanModule, 0, len(modules))
	for _, module := range modules {
		module.Limits = cloneLimits(module.Limits)
		out = append(out, module)
	}
	return out
}

func cloneTenantSubscription(subscription TenantSubscription) *TenantSubscription {
	cloned := subscription
	cloned.StartsAt = cloneTime(subscription.StartsAt)
	cloned.ExpiresAt = cloneTime(subscription.ExpiresAt)
	return &cloned
}
