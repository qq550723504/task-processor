package listingsubscription

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
	domain "task-processor/internal/accountallocation"
)

// ReadTokenQuota exposes only the existing commercial entitlement fact needed
// by member allocation. It is intentionally read-only; allocation mutations
// remain owned by account resources and never write commercial facts.
func (r *GormRepository) ReadTokenQuota(ctx context.Context, organizationID string, at time.Time) (domain.Quota, error) {
	if r == nil || r.db == nil || organizationID == "" || at.IsZero() {
		return domain.Quota{}, domain.ErrQuotaUnavailable
	}
	var row tenantEntitlementRow
	if err := commercialReadTable(r.db.WithContext(ctx), row.TableName()).Where("tenant_id = ? AND module_code = ?", organizationID, ModuleListingKit).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Quota{}, domain.ErrQuotaUnavailable
		}
		return domain.Quota{}, err
	}
	allowed, _ := evaluateEntitlement(&Entitlement{Status: row.Status, StartsAt: row.StartsAt, ExpiresAt: row.ExpiresAt}, at)
	if !allowed || row.StartsAt == nil || row.ExpiresAt == nil {
		return domain.Quota{}, domain.ErrQuotaUnavailable
	}
	var values map[string]int64
	if row.LimitsJSON == "" || json.Unmarshal([]byte(row.LimitsJSON), &values) != nil {
		return domain.Quota{}, domain.ErrQuotaUnavailable
	}
	var total int64
	found := false
	for _, key := range []string{UsageMetricAITokens, "ai_token", domain.MetricToken} {
		if value, ok := values[key]; ok {
			if value < 0 || found && value != total {
				return domain.Quota{}, domain.ErrQuotaUnavailable
			}
			total, found = value, true
		}
	}
	if !found || total == 0 {
		return domain.Quota{}, domain.ErrQuotaUnavailable
	}
	return domain.Quota{OrganizationID: organizationID, Metric: domain.MetricToken, Total: total, WindowStart: row.StartsAt.UTC(), WindowEnd: row.ExpiresAt.UTC()}, nil
}
