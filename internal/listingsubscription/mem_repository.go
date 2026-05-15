package listingsubscription

import (
	"context"
	"sync"
	"time"
)

type MemRepository struct {
	mu           sync.Mutex
	nextID       int64
	modules      map[string]Module
	entitlements map[string]Entitlement
	usage        map[string]UsageCounter
}

func NewMemRepository() *MemRepository {
	return &MemRepository{
		nextID:       1,
		modules:      map[string]Module{},
		entitlements: map[string]Entitlement{},
		usage:        map[string]UsageCounter{},
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
