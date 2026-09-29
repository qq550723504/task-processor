package orgresourceadapter

import (
	"context"
	"encoding/json"
	"strings"

	"task-processor/internal/ledger/orgresource"
)

// ListMemberAudit reads the original transaction's immutable facts. It does
// not use current member positions or limits to reconstruct past operations.
func (r *GormRepository) ListMemberAudit(ctx context.Context, org string, limit int, actor, action string, after *orgresource.MemberAuditPosition) (orgresource.MemberAuditPage, error) {
	validAction := func(value string) bool {
		return value == "allocate_member_resource" || value == "reclaim_member_resource" || value == "set_member_ai_point_limit"
	}
	if ctx == nil || org == "" || strings.TrimSpace(org) != org || len(org) > 128 || len(actor) > 192 || limit < 1 || limit > 100 || action != "" && !validAction(action) || after != nil && (after.ID < 1 || after.CreatedAt.IsZero()) {
		return orgresource.MemberAuditPage{}, orgresource.ErrInvalidInput
	}
	query := r.db.WithContext(ctx).Table("saas_organization_resource_audit_logs AS a").Select("a.*").
		Joins("JOIN saas_organization_resource_operations AS o ON o.organization_id=a.organization_id AND o.operation_id=a.operation_id AND o.operation_type=a.action AND o.state=?", "succeeded").
		Where("a.organization_id=? AND a.action IN ?", org, []string{"allocate_member_resource", "reclaim_member_resource", "set_member_ai_point_limit"})
	if actor != "" {
		query = query.Where("a.actor_id=?", actor)
	}
	if action != "" {
		query = query.Where("a.action=?", action)
	}
	if after != nil {
		query = query.Where("a.created_at<? OR (a.created_at=? AND a.id<?)", after.CreatedAt.UTC(), after.CreatedAt.UTC(), after.ID)
	}
	var rows []organizationResourceAuditLogRow
	if err := query.Order("a.created_at DESC,a.id DESC").Limit(limit + 1).Scan(&rows).Error; err != nil {
		return orgresource.MemberAuditPage{}, err
	}
	page := orgresource.MemberAuditPage{Items: make([]orgresource.MemberAuditEvent, 0, limit)}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		page.Next = &orgresource.MemberAuditPosition{CreatedAt: last.CreatedAt.UTC(), ID: last.ID}
	}
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Action != "set_member_ai_point_limit" {
			keys = append(keys, row.OperationID)
		}
	}
	byOperation := map[string][]organizationResourceEventRow{}
	if len(keys) > 0 {
		var events []organizationResourceEventRow
		if err := r.db.WithContext(ctx).Where("organization_id=? AND operation_id IN ? AND source_type=?", org, keys, "member_resource_position").Find(&events).Error; err != nil {
			return orgresource.MemberAuditPage{}, err
		}
		for _, event := range events {
			byOperation[event.OperationID] = append(byOperation[event.OperationID], event)
		}
	}
	for _, row := range rows {
		item := orgresource.MemberAuditEvent{OrganizationID: org, ID: row.ID, OperationID: row.OperationID, Action: row.Action, ActorID: row.ActorID, CreatedAt: row.CreatedAt.UTC()}
		if row.ID < 1 || row.CreatedAt.IsZero() || row.ActorID == "" || row.OperationID == "" {
			return orgresource.MemberAuditPage{}, orgresource.ErrInvalidInput
		}
		if row.Action == "set_member_ai_point_limit" {
			var receipt orgresource.MemberLimitSnapshot
			if json.Unmarshal([]byte(row.Payload), &receipt) != nil || receipt.OrganizationID != org || receipt.MemberID == "" || receipt.Version < 1 || receipt.MonthlyLimit < 0 {
				return orgresource.MemberAuditPage{}, orgresource.ErrInvalidInput
			}
			item.MemberID, item.ResourceType, item.Quantity, item.Version = receipt.MemberID, orgresource.ResourceAIPoint, receipt.MonthlyLimit, receipt.Version
		} else {
			var receipt orgresource.MemberResourceTransferResult
			if json.Unmarshal([]byte(row.Payload), &receipt) != nil {
				return orgresource.MemberAuditPage{}, orgresource.ErrInvalidInput
			}
			position := receipt.Position
			events := byOperation[row.OperationID]
			if position.OrganizationID != org || position.MemberID == "" || position.Version < 1 || (position.ResourceType != orgresource.ResourceStoreRenewalPeriod && position.ResourceType != orgresource.ResourceDataRow) || len(events) != 1 {
				return orgresource.MemberAuditPage{}, orgresource.ErrInvalidInput
			}
			event := events[0]
			if event.SourceIdentity != position.MemberID || event.ResourceType != string(position.ResourceType) || event.Reason != row.Action || event.Quantity <= 0 {
				return orgresource.MemberAuditPage{}, orgresource.ErrInvalidInput
			}
			item.MemberID, item.ResourceType, item.Quantity, item.Version = position.MemberID, position.ResourceType, event.Quantity, position.Version
		}
		page.Items = append(page.Items, item)
	}
	return page, nil
}
