package listingsubscription

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type subscriptionModuleRow struct {
	Code        string    `gorm:"column:code;primaryKey;size:64"`
	Name        string    `gorm:"column:name;not null"`
	Description string    `gorm:"column:description"`
	SortOrder   int       `gorm:"column:sort_order;not null;default:0"`
	Active      bool      `gorm:"column:active;not null;default:true"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (subscriptionModuleRow) TableName() string { return "saas_modules" }

type subscriptionPlanRow struct {
	Code        string    `gorm:"column:code;primaryKey;size:64"`
	Name        string    `gorm:"column:name;not null"`
	Description string    `gorm:"column:description"`
	SortOrder   int       `gorm:"column:sort_order;not null;default:0"`
	Active      bool      `gorm:"column:active;not null;default:true"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (subscriptionPlanRow) TableName() string { return "saas_plans" }

type subscriptionPlanModuleRow struct {
	PlanCode   string `gorm:"column:plan_code;primaryKey;size:64"`
	ModuleCode string `gorm:"column:module_code;primaryKey;size:64;index"`
	LimitsJSON string `gorm:"column:limits;type:text"`
	SortOrder  int    `gorm:"column:sort_order;not null;default:0"`
}

func (subscriptionPlanModuleRow) TableName() string { return "saas_plan_modules" }

type tenantSubscriptionRow struct {
	ID        int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID  string     `gorm:"column:tenant_id;not null;size:128;uniqueIndex"`
	PlanCode  string     `gorm:"column:plan_code;not null;size:64;index"`
	Status    string     `gorm:"column:status;not null;size:32"`
	StartsAt  *time.Time `gorm:"column:starts_at"`
	ExpiresAt *time.Time `gorm:"column:expires_at"`
	CreatedAt time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (tenantSubscriptionRow) TableName() string { return "saas_tenant_subscriptions" }

type tenantEntitlementRow struct {
	ID         int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID   string     `gorm:"column:tenant_id;not null;size:128;uniqueIndex:idx_saas_tenant_module"`
	ModuleCode string     `gorm:"column:module_code;not null;size:64;uniqueIndex:idx_saas_tenant_module;index"`
	Status     string     `gorm:"column:status;not null;size:32"`
	StartsAt   *time.Time `gorm:"column:starts_at"`
	ExpiresAt  *time.Time `gorm:"column:expires_at"`
	LimitsJSON string     `gorm:"column:limits;type:text"`
	CreatedAt  time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt  time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (tenantEntitlementRow) TableName() string { return "saas_tenant_entitlements" }

type usageCounterRow struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID   string    `gorm:"column:tenant_id;not null;size:128;uniqueIndex:idx_saas_usage_counter"`
	ModuleCode string    `gorm:"column:module_code;not null;size:64;uniqueIndex:idx_saas_usage_counter"`
	PeriodKey  string    `gorm:"column:period_key;not null;size:16;uniqueIndex:idx_saas_usage_counter"`
	Metric     string    `gorm:"column:metric;not null;size:64;uniqueIndex:idx_saas_usage_counter"`
	Used       int       `gorm:"column:used;not null;default:0"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (usageCounterRow) TableName() string { return "saas_usage_counters" }

type auditLogRow struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID   string    `gorm:"column:tenant_id;not null;size:128;index"`
	ModuleCode string    `gorm:"column:module_code;size:64;index"`
	Action     string    `gorm:"column:action;not null;size:64;index"`
	ActorID    string    `gorm:"column:actor_id;size:128;index"`
	Reason     string    `gorm:"column:reason;type:text"`
	Payload    string    `gorm:"column:payload;type:text"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime;index"`
}

func (auditLogRow) TableName() string { return "saas_subscription_audit_logs" }

type GormRepository struct {
	db *gorm.DB
}

func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

func AutoMigrateRepository(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	return db.AutoMigrate(
		&subscriptionModuleRow{},
		&subscriptionPlanRow{},
		&subscriptionPlanModuleRow{},
		&tenantSubscriptionRow{},
		&tenantEntitlementRow{},
		&usageCounterRow{},
		&auditLogRow{},
	)
}

func (r *GormRepository) ListModules(ctx context.Context) ([]Module, error) {
	var rows []subscriptionModuleRow
	if err := r.db.WithContext(ctx).Order("sort_order asc, code asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]Module, 0, len(rows))
	for _, row := range rows {
		items = append(items, Module{
			Code: row.Code, Name: row.Name, Description: row.Description,
			SortOrder: row.SortOrder, Active: row.Active, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	return items, nil
}

func (r *GormRepository) UpsertDefaultModules(ctx context.Context, modules []Module) error {
	for _, module := range modules {
		row := subscriptionModuleRow{
			Code: module.Code, Name: module.Name, Description: module.Description,
			SortOrder: module.SortOrder, Active: module.Active,
		}
		if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "code"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "description", "sort_order", "active", "updated_at"}),
		}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *GormRepository) ListPlans(ctx context.Context) ([]PlanBundle, error) {
	var planRows []subscriptionPlanRow
	if err := r.db.WithContext(ctx).Order("sort_order asc, code asc").Find(&planRows).Error; err != nil {
		return nil, err
	}
	var moduleRows []subscriptionPlanModuleRow
	if err := r.db.WithContext(ctx).Order("sort_order asc, module_code asc").Find(&moduleRows).Error; err != nil {
		return nil, err
	}
	modulesByPlan := make(map[string][]PlanModule)
	for _, row := range moduleRows {
		limits, err := unmarshalLimits(row.LimitsJSON)
		if err != nil {
			return nil, err
		}
		modulesByPlan[row.PlanCode] = append(modulesByPlan[row.PlanCode], PlanModule{
			PlanCode:   row.PlanCode,
			ModuleCode: row.ModuleCode,
			Limits:     limits,
			SortOrder:  row.SortOrder,
		})
	}
	items := make([]PlanBundle, 0, len(planRows))
	for _, row := range planRows {
		items = append(items, PlanBundle{
			Plan: Plan{
				Code: row.Code, Name: row.Name, Description: row.Description,
				SortOrder: row.SortOrder, Active: row.Active, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			},
			Modules: modulesByPlan[row.Code],
		})
	}
	return items, nil
}

func (r *GormRepository) UpsertDefaultPlans(ctx context.Context, plans []PlanBundle) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, bundle := range plans {
			planRow := subscriptionPlanRow{
				Code: bundle.Plan.Code, Name: bundle.Plan.Name, Description: bundle.Plan.Description,
				SortOrder: bundle.Plan.SortOrder, Active: bundle.Plan.Active,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "code"}},
				DoUpdates: clause.AssignmentColumns([]string{"name", "description", "sort_order", "active", "updated_at"}),
			}).Create(&planRow).Error; err != nil {
				return err
			}
			for _, module := range bundle.Modules {
				limitsJSON, err := marshalLimits(module.Limits)
				if err != nil {
					return err
				}
				moduleRow := subscriptionPlanModuleRow{
					PlanCode: bundle.Plan.Code, ModuleCode: module.ModuleCode,
					LimitsJSON: limitsJSON, SortOrder: module.SortOrder,
				}
				if err := tx.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "plan_code"}, {Name: "module_code"}},
					DoUpdates: clause.AssignmentColumns([]string{"limits", "sort_order"}),
				}).Create(&moduleRow).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *GormRepository) UpsertPlan(ctx context.Context, plan Plan, modules []PlanModule) (*PlanBundle, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		planRow := subscriptionPlanRow{
			Code: plan.Code, Name: plan.Name, Description: plan.Description,
			SortOrder: plan.SortOrder, Active: plan.Active,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "code"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "description", "sort_order", "active", "updated_at"}),
		}).Create(&planRow).Error; err != nil {
			return err
		}
		if err := tx.Where("plan_code = ?", plan.Code).Delete(&subscriptionPlanModuleRow{}).Error; err != nil {
			return err
		}
		for _, module := range modules {
			limitsJSON, err := marshalLimits(module.Limits)
			if err != nil {
				return err
			}
			row := subscriptionPlanModuleRow{
				PlanCode: plan.Code, ModuleCode: module.ModuleCode,
				LimitsJSON: limitsJSON, SortOrder: module.SortOrder,
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.getPlanBundle(ctx, plan.Code)
}

func (r *GormRepository) UpsertPlanModule(ctx context.Context, module PlanModule) (*PlanBundle, error) {
	limitsJSON, err := marshalLimits(module.Limits)
	if err != nil {
		return nil, err
	}
	row := subscriptionPlanModuleRow{
		PlanCode: module.PlanCode, ModuleCode: module.ModuleCode,
		LimitsJSON: limitsJSON, SortOrder: module.SortOrder,
	}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "plan_code"}, {Name: "module_code"}},
		DoUpdates: clause.AssignmentColumns([]string{"limits", "sort_order"}),
	}).Create(&row).Error; err != nil {
		return nil, err
	}
	return r.getPlanBundle(ctx, module.PlanCode)
}

func (r *GormRepository) DeletePlanModule(ctx context.Context, planCode, moduleCode string) (*PlanBundle, error) {
	if err := r.db.WithContext(ctx).
		Where("plan_code = ? AND module_code = ?", planCode, moduleCode).
		Delete(&subscriptionPlanModuleRow{}).Error; err != nil {
		return nil, err
	}
	return r.getPlanBundle(ctx, planCode)
}

func (r *GormRepository) GetTenantSubscription(ctx context.Context, tenantID string) (*TenantSubscription, error) {
	var row tenantSubscriptionRow
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrEntitlementNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toTenantSubscription(), nil
}

func (r *GormRepository) ListTenantSubscriptionsByPlan(ctx context.Context, planCode string) ([]TenantSubscription, error) {
	var rows []tenantSubscriptionRow
	if err := r.db.WithContext(ctx).Where("plan_code = ?", planCode).Order("updated_at DESC, tenant_id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]TenantSubscription, 0, len(rows))
	for _, row := range rows {
		items = append(items, *row.toTenantSubscription())
	}
	return items, nil
}

func (r *GormRepository) UpsertTenantSubscription(ctx context.Context, subscription *TenantSubscription) (*TenantSubscription, error) {
	row := tenantSubscriptionRow{
		TenantID: subscription.TenantID, PlanCode: subscription.PlanCode, Status: subscription.Status,
		StartsAt: subscription.StartsAt, ExpiresAt: subscription.ExpiresAt,
	}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"plan_code":  row.PlanCode,
			"status":     row.Status,
			"starts_at":  row.StartsAt,
			"expires_at": row.ExpiresAt,
			"updated_at": time.Now().UTC(),
		}),
	}).Create(&row).Error; err != nil {
		return nil, err
	}
	return r.GetTenantSubscription(ctx, subscription.TenantID)
}

func (r *GormRepository) GetEntitlement(ctx context.Context, tenantID, moduleCode string) (*Entitlement, error) {
	var row tenantEntitlementRow
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND module_code = ?", tenantID, moduleCode).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrEntitlementNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toEntitlement()
}

func (r *GormRepository) ListEntitlements(ctx context.Context, tenantID string) ([]Entitlement, error) {
	var rows []tenantEntitlementRow
	if err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("module_code asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]Entitlement, 0, len(rows))
	for _, row := range rows {
		entitlement, err := row.toEntitlement()
		if err != nil {
			return nil, err
		}
		items = append(items, *entitlement)
	}
	return items, nil
}

func (r *GormRepository) ListTenantOverviews(ctx context.Context) ([]TenantOverview, error) {
	var rows []struct {
		TenantID         string     `gorm:"column:tenant_id"`
		EntitlementCount int        `gorm:"column:entitlement_count"`
		ActiveCount      int        `gorm:"column:active_count"`
		UpdatedAt        *time.Time `gorm:"column:updated_at"`
	}
	if err := r.db.WithContext(ctx).
		Model(&tenantEntitlementRow{}).
		Select("tenant_id, COUNT(*) AS entitlement_count, SUM(CASE WHEN status IN ? THEN 1 ELSE 0 END) AS active_count, MAX(updated_at) AS updated_at", []string{StatusActive, StatusTrialing}).
		Group("tenant_id").
		Order("updated_at DESC, tenant_id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]TenantOverview, 0, len(rows))
	for _, row := range rows {
		items = append(items, TenantOverview{
			TenantID:         row.TenantID,
			EntitlementCount: row.EntitlementCount,
			ActiveCount:      row.ActiveCount,
			UpdatedAt:        row.UpdatedAt,
		})
	}
	return items, nil
}

func (r *GormRepository) UpsertEntitlement(ctx context.Context, entitlement *Entitlement) (*Entitlement, error) {
	limitsJSON, err := marshalLimits(entitlement.Limits)
	if err != nil {
		return nil, err
	}
	row := tenantEntitlementRow{
		TenantID: entitlement.TenantID, ModuleCode: entitlement.ModuleCode, Status: entitlement.Status,
		StartsAt: entitlement.StartsAt, ExpiresAt: entitlement.ExpiresAt, LimitsJSON: limitsJSON,
	}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "module_code"}},
		DoUpdates: clause.Assignments(map[string]any{
			"status":     row.Status,
			"starts_at":  row.StartsAt,
			"expires_at": row.ExpiresAt,
			"limits":     row.LimitsJSON,
			"updated_at": time.Now().UTC(),
		}),
	}).Create(&row).Error; err != nil {
		return nil, err
	}
	return r.GetEntitlement(ctx, entitlement.TenantID, entitlement.ModuleCode)
}

func (r *GormRepository) ListUsage(ctx context.Context, tenantID string) ([]UsageCounter, error) {
	var rows []usageCounterRow
	if err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("module_code asc, metric asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]UsageCounter, 0, len(rows))
	for _, row := range rows {
		items = append(items, UsageCounter{
			ID: row.ID, TenantID: row.TenantID, ModuleCode: row.ModuleCode,
			PeriodKey: row.PeriodKey, Metric: row.Metric, Used: row.Used, UpdatedAt: row.UpdatedAt,
		})
	}
	return items, nil
}

func (r *GormRepository) IncrementUsage(ctx context.Context, tenantID, moduleCode, periodKey, metric string, amount int) (*UsageCounter, error) {
	row := usageCounterRow{TenantID: tenantID, ModuleCode: moduleCode, PeriodKey: periodKey, Metric: metric, Used: amount}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "module_code"}, {Name: "period_key"}, {Name: "metric"}},
		DoUpdates: clause.Assignments(map[string]any{
			"used":       gorm.Expr("used + ?", amount),
			"updated_at": time.Now().UTC(),
		}),
	}).Create(&row).Error; err != nil {
		return nil, err
	}
	var out usageCounterRow
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND module_code = ? AND period_key = ? AND metric = ?", tenantID, moduleCode, periodKey, metric).Take(&out).Error; err != nil {
		return nil, err
	}
	return &UsageCounter{ID: out.ID, TenantID: out.TenantID, ModuleCode: out.ModuleCode, PeriodKey: out.PeriodKey, Metric: out.Metric, Used: out.Used, UpdatedAt: out.UpdatedAt}, nil
}

func (r *GormRepository) SetUsage(ctx context.Context, tenantID, moduleCode, periodKey, metric string, used int) (*UsageCounter, error) {
	row := usageCounterRow{TenantID: tenantID, ModuleCode: moduleCode, PeriodKey: periodKey, Metric: metric, Used: used}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "module_code"}, {Name: "period_key"}, {Name: "metric"}},
		DoUpdates: clause.Assignments(map[string]any{
			"used":       used,
			"updated_at": time.Now().UTC(),
		}),
	}).Create(&row).Error; err != nil {
		return nil, err
	}
	var out usageCounterRow
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND module_code = ? AND period_key = ? AND metric = ?", tenantID, moduleCode, periodKey, metric).Take(&out).Error; err != nil {
		return nil, err
	}
	return &UsageCounter{ID: out.ID, TenantID: out.TenantID, ModuleCode: out.ModuleCode, PeriodKey: out.PeriodKey, Metric: out.Metric, Used: out.Used, UpdatedAt: out.UpdatedAt}, nil
}

func (r *GormRepository) CreateAuditLog(ctx context.Context, log AuditLog) (*AuditLog, error) {
	row := auditLogRow{TenantID: log.TenantID, ModuleCode: log.ModuleCode, Action: log.Action, ActorID: log.ActorID, Reason: log.Reason, Payload: log.Payload}
	if !log.CreatedAt.IsZero() {
		row.CreatedAt = log.CreatedAt
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, err
	}
	return &AuditLog{ID: row.ID, TenantID: row.TenantID, ModuleCode: row.ModuleCode, Action: row.Action, ActorID: row.ActorID, Reason: row.Reason, Payload: row.Payload, CreatedAt: row.CreatedAt}, nil
}

func (r *GormRepository) ListAuditLogs(ctx context.Context, tenantID string, limit int) ([]AuditLog, error) {
	var rows []auditLogRow
	if err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at DESC, id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]AuditLog, 0, len(rows))
	for _, row := range rows {
		items = append(items, AuditLog{ID: row.ID, TenantID: row.TenantID, ModuleCode: row.ModuleCode, Action: row.Action, ActorID: row.ActorID, Reason: row.Reason, Payload: row.Payload, CreatedAt: row.CreatedAt})
	}
	return items, nil
}

func (r *GormRepository) ListPlanAuditLogs(ctx context.Context, planCode string, limit int) ([]AuditLog, error) {
	var rows []auditLogRow
	if err := r.db.WithContext(ctx).Where("reason = ?", planCode).Order("created_at DESC, id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]AuditLog, 0, len(rows))
	for _, row := range rows {
		items = append(items, AuditLog{ID: row.ID, TenantID: row.TenantID, ModuleCode: row.ModuleCode, Action: row.Action, ActorID: row.ActorID, Reason: row.Reason, Payload: row.Payload, CreatedAt: row.CreatedAt})
	}
	return items, nil
}

func (r *GormRepository) getPlanBundle(ctx context.Context, planCode string) (*PlanBundle, error) {
	var row subscriptionPlanRow
	err := r.db.WithContext(ctx).Where("code = ?", planCode).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrModuleNotFound
	}
	if err != nil {
		return nil, err
	}
	var moduleRows []subscriptionPlanModuleRow
	if err := r.db.WithContext(ctx).Where("plan_code = ?", planCode).Order("sort_order asc, module_code asc").Find(&moduleRows).Error; err != nil {
		return nil, err
	}
	modules := make([]PlanModule, 0, len(moduleRows))
	for _, moduleRow := range moduleRows {
		limits, err := unmarshalLimits(moduleRow.LimitsJSON)
		if err != nil {
			return nil, err
		}
		modules = append(modules, PlanModule{
			PlanCode:   moduleRow.PlanCode,
			ModuleCode: moduleRow.ModuleCode,
			Limits:     limits,
			SortOrder:  moduleRow.SortOrder,
		})
	}
	return &PlanBundle{
		Plan: Plan{
			Code: row.Code, Name: row.Name, Description: row.Description,
			SortOrder: row.SortOrder, Active: row.Active, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		},
		Modules: modules,
	}, nil
}

func (row tenantEntitlementRow) toEntitlement() (*Entitlement, error) {
	limits, err := unmarshalLimits(row.LimitsJSON)
	if err != nil {
		return nil, err
	}
	return &Entitlement{
		ID: row.ID, TenantID: row.TenantID, ModuleCode: row.ModuleCode, Status: row.Status,
		StartsAt: row.StartsAt, ExpiresAt: row.ExpiresAt, Limits: limits, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func (row tenantSubscriptionRow) toTenantSubscription() *TenantSubscription {
	return &TenantSubscription{
		ID: row.ID, TenantID: row.TenantID, PlanCode: row.PlanCode, Status: row.Status,
		StartsAt: row.StartsAt, ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func marshalLimits(limits map[string]int) (string, error) {
	if len(limits) == 0 {
		return "{}", nil
	}
	data, err := json.Marshal(limits)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unmarshalLimits(value string) (map[string]int, error) {
	if value == "" {
		return nil, nil
	}
	var limits map[string]int
	if err := json.Unmarshal([]byte(value), &limits); err != nil {
		return nil, err
	}
	return cloneLimits(limits), nil
}
