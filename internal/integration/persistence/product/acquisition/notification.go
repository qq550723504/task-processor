package acquisition

import (
	"context"
	"github.com/google/uuid"
	"task-processor/internal/product/sourcing"
)

func (r *Repository) ListNoticeOperations(ctx context.Context, scope sourcing.PublicationScope, after string, limit int) ([]sourcing.AcquisitionOperation, string, error) {
	if limit < 1 || limit > 100 || after != "" && uuid.Validate(after) != nil {
		return nil, "", sourcing.ErrInvalidAcquisition
	}
	q := r.db.WithContext(ctx).Table(table).Select("operation_id").Where("organization_id=? AND actor_id=?", scope.OrganizationID, scope.ActorID)
	if after != "" {
		q = q.Where("operation_id>?", after)
	}
	var rows []struct{ OperationID string }
	if e := q.Order("operation_id ASC").Limit(limit + 1).Find(&rows).Error; e != nil {
		return nil, "", sourcing.ErrAcquisitionUnavailable
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = rows[len(rows)-1].OperationID
	}
	result := make([]sourcing.AcquisitionOperation, 0, len(rows))
	for _, row := range rows {
		op, e := r.ByID(ctx, scope, row.OperationID)
		if e != nil {
			return nil, "", e
		}
		result = append(result, op)
	}
	return result, next, nil
}
