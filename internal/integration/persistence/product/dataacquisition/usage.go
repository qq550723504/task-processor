package dataacquisitionpersistence

import (
	"context"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/dataacquisition"
)

func (r *Repository) Usage(ctx context.Context, s collection.Scope) (dataacquisition.Usage, error) {
	if s.Validate() != nil {
		return dataacquisition.Usage{}, dataacquisition.ErrForbidden
	}
	var usage dataacquisition.Usage
	err := r.db.WithContext(ctx).Raw(`SELECT
 (SELECT count(*) FROM data_acquisition_items WHERE organization_id=? AND actor_id=? AND state='SAVED' AND saved_at>=date_trunc('day',now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC') AS day_rows,
 (SELECT count(*)*5 FROM data_acquisition_items WHERE organization_id=? AND actor_id=? AND state='SAVED' AND charge_state='committed' AND saved_at>=date_trunc('month',now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC') AS month_confirmed_fen,
 (SELECT count(*)*5 FROM data_acquisition_items WHERE organization_id=? AND actor_id=? AND state='SAVED' AND charge_state<>'committed' AND saved_at>=date_trunc('month',now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC') AS month_pending_fen,
 count(*) FILTER (WHERE state IN ('SUCCEEDED','PARTIAL','FAILED','CANCELED')) AS finished_jobs,
 count(*) FILTER (WHERE state='SUCCEEDED') AS succeeded_jobs
 FROM data_acquisition_jobs WHERE organization_id=? AND actor_id=? AND created_at>=date_trunc('month',now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'`, s.OrganizationID, s.ActorID, s.OrganizationID, s.ActorID, s.OrganizationID, s.ActorID, s.OrganizationID, s.ActorID).Scan(&usage).Error
	if err != nil {
		return usage, err
	}
	usage.Window = "UTC calendar month; completed jobs"
	if usage.FinishedJobs > 0 {
		ratio := float64(usage.SucceededJobs) / float64(usage.FinishedJobs)
		usage.SuccessRate = &ratio
	}
	return usage, nil
}
func (r *Repository) KeyQuotas(ctx context.Context, s collection.Scope) ([]dataacquisition.KeyQuota, error) {
	if s.Validate() != nil {
		return nil, dataacquisition.ErrForbidden
	}
	out := []dataacquisition.KeyQuota{}
	err := r.db.WithContext(ctx).Raw(`SELECT k.id AS key_id,
 COALESCE(sum(q.consumed_rows) FILTER(WHERE q.window_kind='day'),0) AS day_consumed_rows,
 COALESCE(sum(q.reserved_rows) FILTER(WHERE q.window_kind='day'),0) AS day_reserved_rows,
 COALESCE(sum(q.consumed_fen) FILTER(WHERE q.window_kind='month'),0) AS month_consumed_fen,
 COALESCE(sum(q.reserved_fen) FILTER(WHERE q.window_kind='month'),0) AS month_reserved_fen
 FROM data_service_credentials k LEFT JOIN data_service_quota q ON q.organization_id=k.organization_id AND q.actor_id=k.actor_id AND q.key_id=k.id AND q.window_start=date_trunc(q.window_kind,now() AT TIME ZONE 'UTC')::date
 WHERE k.organization_id=? AND k.actor_id=? AND k.state<>'REVOKED' AND k.expires_at>now() GROUP BY k.id ORDER BY k.id`, s.OrganizationID, s.ActorID).Scan(&out).Error
	return out, err
}
