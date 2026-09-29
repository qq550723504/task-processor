package storecenter

import (
	"context"
	"time"
)

type ServiceSummary struct {
	Records      int64 `json:"records"`
	Active       int64 `json:"active"`
	Expired      int64 `json:"expired"`
	ExpiringSoon int64 `json:"expiring_soon"`
}

// Aggregate counts disclose no member-scoped Store identity or connection data.
func (r *GormStoreRepository) ReadServiceSummary(ctx context.Context, organization string, now time.Time) (ServiceSummary, error) {
	if _, err := validateOpaqueIdentity("organization", organization, MaxOrganizationIDBytes); err != nil || now.IsZero() {
		return ServiceSummary{}, ErrNotFound
	}
	var result ServiceSummary
	err := r.db.WithContext(ctx).Model(&workbenchStoreRecord{}).
		Where("organization_id=? AND record_status IN ('active','disabled') AND deleted_at IS NULL", organization).
		Select(`count(*) AS records,
		coalesce(sum(CASE WHEN record_status='active' AND service_status='active' AND service_started_at<=? AND service_expires_at>? THEN 1 ELSE 0 END),0) AS active,
		coalesce(sum(CASE WHEN record_status='active' AND service_status IN ('active','expired') AND service_expires_at<=? THEN 1 ELSE 0 END),0) AS expired,
		coalesce(sum(CASE WHEN record_status='active' AND service_status='active' AND service_started_at<=? AND service_expires_at>? AND service_expires_at<=? THEN 1 ELSE 0 END),0) AS expiring_soon`, now, now, now, now, now, now.Add(7*24*time.Hour)).Scan(&result).Error
	return result, err
}
