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
	return db.AutoMigrate(&subscriptionModuleRow{}, &tenantEntitlementRow{}, &usageCounterRow{})
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
