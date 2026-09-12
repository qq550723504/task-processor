package sourceaccountregistry

import (
	"context"
	"strings"
	registry "task-processor/internal/sourceaccountregistry"
	"time"
	"unicode/utf8"
)

// ListCommittedOperations reads only immutable registry receipts. It never
// joins current resource state, reads request payloads, or performs a write.
func (r *Repository) ListCommittedOperations(ctx context.Context, organizationID string, request registry.HistoryRequest) (registry.HistoryPage, error) {
	if r == nil || r.db == nil {
		return registry.HistoryPage{}, registry.ErrUnavailable
	}
	if organizationID == "" || len(organizationID) > registry.MaxOrganizationIDBytes || strings.TrimSpace(organizationID) != organizationID || !utf8.ValidString(organizationID) || request.Validate() != nil {
		return registry.HistoryPage{}, registry.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, registry.Timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return registry.HistoryPage{}, err
	}
	query := r.db.WithContext(ctx).Table(operationTable).Select("organization_id, actor_subject, kind, account_id, resulting_version, created_at").Where("organization_id = ?", organizationID)
	if request.After != nil {
		p := request.After
		query = query.Where("(created_at, account_id, resulting_version) < (?, ?::uuid, ?)", p.OccurredAt.UTC(), p.AccountID, p.Version)
	}
	var rows []operationRow
	if err := query.Order("created_at DESC, account_id DESC, resulting_version DESC").Limit(request.Limit + 1).Find(&rows).Error; err != nil {
		return registry.HistoryPage{}, mapError(ctx, err)
	}
	if err := ctx.Err(); err != nil {
		return registry.HistoryPage{}, err
	}
	page := registry.HistoryPage{Items: make([]registry.CommittedOperation, 0, min(request.Limit, len(rows)))}
	var previous *registry.HistoryPosition = request.After
	for _, row := range rows {
		item := registry.CommittedOperation{OrganizationID: row.OrganizationID, AccountID: row.AccountID, ActorSubject: row.ActorSubject, Kind: registry.OperationKind(row.Kind), Version: row.ResultingVersion, OccurredAt: row.CreatedAt.UTC().Truncate(time.Microsecond)}
		p := item.Position()
		if item.Validate() != nil || item.OrganizationID != organizationID || previous != nil && !p.Before(*previous) {
			return registry.HistoryPage{}, registry.ErrUnavailable
		}
		previous = &p
		if len(page.Items) == request.Limit {
			last := page.Items[len(page.Items)-1].Position()
			page.Next = &last
			break
		}
		page.Items = append(page.Items, item)
	}
	return page, nil
}
