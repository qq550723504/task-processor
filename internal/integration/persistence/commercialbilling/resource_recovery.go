package commercialbilling

import (
	"context"
	"task-processor/internal/commercial/billing"
)

func (r *Repository) ListRecoverableResourceOrders(ctx context.Context, after *billing.ResourceRecoveryPosition, limit int) (billing.ResourceRecoveryPage, error) {
	if r == nil || r.db == nil || limit < 1 || limit > 50 || after != nil && (after.CreatedAt.IsZero() || after.OrderID == "" || len(after.OrderID) > 128) {
		return billing.ResourceRecoveryPage{}, billing.ErrInvalid
	}
	query := r.db.WithContext(ctx).Where("kind = ? AND status IN ?", string(billing.OrderResourcePurchase), []string{string(billing.OrderPending), string(billing.OrderFundsReserved), string(billing.OrderFulfilling), string(billing.OrderReconciliationRequired)})
	if after != nil {
		query = query.Where("created_at > ? OR (created_at = ? AND order_id > ?)", after.CreatedAt.UTC(), after.CreatedAt.UTC(), after.OrderID)
	}
	var rows []orderRow
	if query.Order("created_at ASC, order_id ASC").Limit(limit+1).Find(&rows).Error != nil {
		return billing.ResourceRecoveryPage{}, billing.ErrFeatureUnavailable
	}
	page := billing.ResourceRecoveryPage{Orders: make([]billing.Order, 0, len(rows))}
	if len(rows) > limit {
		last := rows[limit-1]
		page.Next = &billing.ResourceRecoveryPosition{CreatedAt: last.CreatedAt, OrderID: last.OrderID}
		rows = rows[:limit]
	}
	for _, row := range rows {
		var item orderItemRow
		if r.db.WithContext(ctx).Where("order_id = ?", row.OrderID).Take(&item).Error != nil {
			return billing.ResourceRecoveryPage{}, billing.ErrFeatureUnavailable
		}
		order := orderFromRows(row, item)
		if order.Validate() != nil || order.ActorID == "" {
			return billing.ResourceRecoveryPage{}, billing.ErrFeatureUnavailable
		}
		page.Orders = append(page.Orders, order)
	}
	return page, nil
}
