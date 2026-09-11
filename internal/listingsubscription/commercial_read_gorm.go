package listingsubscription

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gorm.io/gorm"
)

var commercialModules = []string{ModuleStoreManagement, ModuleTaskImport, ModuleRules, ModuleOperationStrategy, ModuleListingKit, ModuleOSSStorage}
var commercialMetrics = []string{usageMetricListingKitGenerationsSucceeded, usageMetricProductImageJobsSucceeded, usageMetricSheinDraftsSucceeded, usageMetricSheinPublishesSucceeded, usageMetricStorageBytesCurrent}

// Commercial PostgreSQL reads must address exactly the public facts admitted by
// VerifyCommercialReadSchema, independent of each connection's search_path.
// SQLite has no public schema. This is dialect naming, never error fallback.
func commercialReadTable(tx *gorm.DB, table string) *gorm.DB {
	if tx.Dialector.Name() == "postgres" {
		table = "public." + table
	}
	return tx.Table(table)
}

func (r *GormRepository) ReadCommercialOverview(ctx context.Context, organizationID string, at time.Time) (*CommercialOverview, error) {
	if r == nil || r.db == nil || !commercialID.MatchString(organizationID) || at.IsZero() {
		return nil, ErrCommercialUnavailable
	}
	result := &CommercialOverview{OrganizationID: organizationID, ObservedAt: at.UTC(), Entitlements: []CommercialEntitlement{}, Usage: []CommercialUsage{},
		Plans:           []CommercialPlanOption{{Code: "base_payg", Name: "基础方案 · 按需使用", Source: "approved_product_description", Availability: "not_for_sale"}},
		ResourceBalance: UnsupportedCommercialValue{State: "unsupported"}, CashBalance: UnsupportedCommercialValue{State: "unsupported"}}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("SET LOCAL statement_timeout = '10s'").Error; err != nil {
				return err
			}
		}
		if err := readCommercialSubscription(tx, result, at); err != nil {
			return err
		}
		var grants []tenantEntitlementRow
		// Bound both rows and stored JSON before transferring untrusted text.
		if err := commercialReadTable(tx, tenantEntitlementRow{}.TableName()).Select("module_code, status, starts_at, expires_at, updated_at, substr(limits, 1, 4097) AS limits").Where("tenant_id = ? AND module_code IN ?", organizationID, commercialModules).Order("module_code").Limit(7).Find(&grants).Error; err != nil {
			return err
		}
		if len(grants) > 6 {
			return ErrCommercialUnavailable
		}
		for _, grant := range grants {
			effective, err := commercialEffectiveStatus(grant.Status, grant.StartsAt, grant.ExpiresAt, at)
			if err != nil || grant.UpdatedAt.IsZero() {
				return ErrCommercialUnavailable
			}
			limits, unknown, err := commercialLimits(grant.ModuleCode, grant.LimitsJSON)
			if err != nil {
				return err
			}
			result.Entitlements = append(result.Entitlements, CommercialEntitlement{ModuleCode: grant.ModuleCode, Status: grant.Status, EffectiveStatus: effective, StartsAt: commercialUTC(grant.StartsAt), ExpiresAt: commercialUTC(grant.ExpiresAt), UpdatedAt: grant.UpdatedAt.UTC(), LimitsScope: "explicit_grant_only", Limits: limits, UninterpretedLimitCount: unknown})
		}
		return readCommercialUsage(tx, result, at)
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func readCommercialSubscription(tx *gorm.DB, result *CommercialOverview, at time.Time) error {
	var row tenantSubscriptionRow
	err := commercialReadTable(tx, row.TableName()).Select("substr(plan_code, 1, 129) AS plan_code, status, starts_at, expires_at, updated_at").Where("tenant_id = ?", result.OrganizationID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !commercialText(row.PlanCode, 128) || row.UpdatedAt.IsZero() {
		return ErrCommercialUnavailable
	}
	effective, err := commercialEffectiveStatus(row.Status, row.StartsAt, row.ExpiresAt, at)
	if err != nil {
		return err
	}
	value := &CommercialSubscription{PlanCode: row.PlanCode, Status: row.Status, EffectiveStatus: effective, StartsAt: commercialUTC(row.StartsAt), ExpiresAt: commercialUTC(row.ExpiresAt), UpdatedAt: row.UpdatedAt.UTC()}
	var plan subscriptionPlanRow
	err = commercialReadTable(tx, plan.TableName()).Select("substr(name, 1, 257) AS name").Where("code = ?", row.PlanCode).Take(&plan).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err == nil {
		if !commercialText(plan.Name, 256) {
			return ErrCommercialUnavailable
		}
		value.PlanName = &plan.Name
	}
	result.Subscription = value
	if row.PlanCode == "paid_pilot" {
		result.Plans = append(result.Plans, CommercialPlanOption{Code: "paid_pilot", Name: "付费试点", Source: "approved_product_description", Availability: "invitation_only"})
	}
	return nil
}

func commercialEffectiveStatus(status string, starts, expires *time.Time, at time.Time) (string, error) {
	switch status {
	case StatusActive, StatusTrialing, StatusExpired, StatusDisabled:
	default:
		return "", ErrCommercialUnavailable
	}
	if (starts != nil && starts.IsZero()) || (expires != nil && expires.IsZero()) || (starts != nil && expires != nil && !starts.Before(*expires)) {
		return "", ErrCommercialUnavailable
	}
	allowed, reason := evaluateEntitlement(&Entitlement{Status: status, StartsAt: starts, ExpiresAt: expires}, at)
	if allowed {
		return status, nil
	}
	return reason, nil
}

func commercialLimits(module, raw string) ([]CommercialLimit, int, error) {
	limits := []CommercialLimit{}
	values := map[string]int64{}
	if len(raw) > 4096 {
		return nil, 0, ErrCommercialUnavailable
	}
	if raw != "" && raw != "null" {
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			return nil, 0, ErrCommercialUnavailable
		}
	}
	if len(values) > 64 {
		return nil, 0, ErrCommercialUnavailable
	}
	for _, value := range values {
		if value < 0 {
			return nil, 0, ErrCommercialUnavailable
		}
	}
	keys := commercialMetrics
	if module == ModuleStoreManagement {
		keys = []string{"store_count"}
	}
	interpreted := map[string]bool{}
	for _, metric := range keys {
		if metric != "store_count" && !usageMetricModuleMatches(module, metric) {
			continue
		}
		var chosen string
		for _, key := range usageMetricLimitKeys(metric) {
			if _, ok := values[key]; ok {
				interpreted[key] = true
				if chosen == "" {
					chosen = key
				}
			}
		}
		if chosen == "" {
			continue
		}
		unit := "operation"
		if metric == "store_count" {
			unit = "store"
		}
		if metric == usageMetricStorageBytesCurrent {
			unit = "byte"
		}
		decimal := strconv.FormatInt(values[chosen], 10)
		limit := CommercialLimit{Metric: metric, SourceKey: chosen, Unit: unit, RawValue: decimal, Kind: "finite", Value: &decimal}
		if values[chosen] == 0 && metric != "store_count" {
			limit.Kind, limit.Value = "unlimited", nil
		}
		limits = append(limits, limit)
	}
	return limits, len(values) - len(interpreted), nil
}

func readCommercialUsage(tx *gorm.DB, result *CommercialOverview, at time.Time) error {
	start := time.Date(at.UTC().Year(), at.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	for _, metric := range commercialMetrics {
		usage := CommercialUsage{ModuleCode: ModuleListingKit, Metric: metric, Source: "subscription_usage_ledger", Unit: "operation", PeriodKey: start.Format("2006-01"), WindowStart: &start, WindowEnd: &end, State: "unknown"}
		if metric == usageMetricStorageBytesCurrent {
			usage.ModuleCode, usage.Unit, usage.PeriodKey, usage.WindowStart, usage.WindowEnd = ModuleOSSStorage, "byte", usageStorageBucketPeriodKey, nil, nil
		}
		var bucket usageBucketRow
		err := commercialReadTable(tx, bucket.TableName()).Where("tenant_id = ? AND module_code = ? AND metric = ? AND period_key = ?", result.OrganizationID, usage.ModuleCode, metric, usage.PeriodKey).Take(&bucket).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			if validateUsageBucketTotals(metric, bucket.Committed, bucket.Reserved) != nil || bucket.UpdatedAt.IsZero() {
				return ErrCommercialUnavailable
			}
			committed, reserved := strconv.FormatInt(bucket.Committed, 10), strconv.FormatInt(bucket.Reserved, 10)
			usage.State, usage.Committed, usage.Reserved, usage.UpdatedAt = "known", &committed, &reserved, commercialUTC(&bucket.UpdatedAt)
		}
		result.Usage = append(result.Usage, usage)
	}
	return nil
}

func commercialUTC(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	result := value.UTC()
	return &result
}
func commercialText(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && utf8.ValidString(value) && strings.TrimSpace(value) == value && !strings.ContainsFunc(value, unicode.IsControl)
}
