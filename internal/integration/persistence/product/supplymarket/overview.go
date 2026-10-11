package supplymarketpersistence

import (
	"context"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/supplymarket"
)

func (r *Repository) CountApplications(ctx context.Context, scope collection.Scope) (supplymarket.ApplicationCounts, error) {
	var counts supplymarket.ApplicationCounts
	if ctx == nil || r == nil || r.db == nil || scope.Validate() != nil {
		return counts, supplymarket.ErrForbidden
	}
	err := r.db.WithContext(ctx).Raw(`SELECT
 COUNT(*) FILTER(WHERE stage IN ('SUBMITTED','EVALUATING')) AS reviewing,
 COUNT(*) FILTER(WHERE stage='SUPPLEMENT_REQUIRED') AS supplement_required,
 COUNT(*) FILTER(WHERE stage='APPROVED') AS approved
 FROM supply_market_records
 WHERE organization_id=? AND actor_id=? AND member_id=? AND kind='selected'`, scope.OrganizationID, scope.ActorID, scope.MemberID).Scan(&counts).Error
	if err != nil {
		return supplymarket.ApplicationCounts{}, err
	}
	return counts, ctx.Err()
}
