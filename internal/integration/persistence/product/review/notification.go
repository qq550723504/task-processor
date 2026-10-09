package reviewpersistence

import (
	"context"
	"task-processor/internal/product/review"
)

func (r *Repository) ListNotificationRecords(ctx context.Context, scope review.Scope, after string, limit int) ([]review.Record, string, error) {
	if limit < 1 || limit > 100 {
		return nil, "", review.ErrInvalid
	}
	q := r.db.WithContext(ctx).Where("org=?", scope.Org)
	if !scope.Admin {
		q = q.Where("owner=?", scope.Actor)
	}
	if after != "" {
		q = q.Where("id>?", after)
	}
	var rows []proposalRow
	if e := q.Select("org,id,owner,state,CASE WHEN octet_length(payload)<=? THEN payload ELSE NULL END AS payload", review.MaxRecordBytes).Order("id ASC").Limit(limit + 1).Find(&rows).Error; e != nil {
		return nil, "", review.ErrUnavailable
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = rows[len(rows)-1].ID
	}
	result := make([]review.Record, 0, len(rows))
	for _, row := range rows {
		record, e := decodeProposalRow(row)
		if e != nil {
			return nil, "", review.ErrUnavailable
		}
		result = append(result, record)
	}
	return result, next, nil
}
