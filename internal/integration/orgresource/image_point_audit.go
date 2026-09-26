package orgresourceadapter

import (
	"context"
	"strings"

	"task-processor/internal/ledger/orgresource"
)

// ListImagePointDebits reads only committed image-generation resource events.
// All joins retain organization identity; current balance, current member
// configuration and current workflow state cannot rewrite this history.
func (r *GormRepository) ListImagePointDebits(ctx context.Context, organization, actor string, limit int, after *orgresource.ImagePointAuditPosition) (orgresource.ImagePointAuditPage, error) {
	if strings.TrimSpace(organization) == "" || len(organization) > 128 || len(actor) > 192 || limit < 1 || limit > 100 || after != nil && (after.CreatedAt.IsZero() || after.EventID == "" || len(after.EventID) > 128) {
		return orgresource.ImagePointAuditPage{}, orgresource.ErrInvalidInput
	}
	query := r.db.WithContext(ctx).Table("saas_organization_resource_events AS e").
		Select("e.organization_id, e.event_id, a.actor_id, r.member_id, r.business_scope AS run_id, e.source_identity AS intent_id, r.price_version, e.quantity AS points, e.created_at").
		Joins("JOIN saas_organization_resource_reservations AS r ON r.organization_id = e.organization_id AND r.reservation_id = e.reservation_id AND r.settlement_operation_id = e.operation_id AND r.owner_attempt_id = e.source_identity AND r.resource_type = e.resource_type AND r.quantity = e.quantity").
		Joins("JOIN saas_organization_resource_audit_logs AS a ON a.organization_id = e.organization_id AND a.operation_id = e.operation_id AND a.action = ?", "image_generation_finalize:committed").
		Where("e.organization_id = ? AND e.resource_type = ? AND e.source_type = ? AND e.consumed_delta = e.quantity AND e.reason = ? AND r.owner_type = ? AND r.state = ?", organization, "ai_point", "image_generation_v1", "commit", "image_generation_v1", "committed")
	if actor != "" {
		query = query.Where("a.actor_id = ?", actor)
	}
	if after != nil {
		query = query.Where("e.created_at < ? OR (e.created_at = ? AND e.event_id < ?)", after.CreatedAt.UTC(), after.CreatedAt.UTC(), after.EventID)
	}
	var items []orgresource.ImagePointDebit
	if err := query.Order("e.created_at DESC, e.event_id DESC").Limit(limit + 1).Scan(&items).Error; err != nil {
		return orgresource.ImagePointAuditPage{}, err
	}
	page := orgresource.ImagePointAuditPage{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		last := page.Items[limit-1]
		page.Next = &orgresource.ImagePointAuditPosition{CreatedAt: last.CreatedAt, EventID: last.EventID}
	}
	return page, nil
}
