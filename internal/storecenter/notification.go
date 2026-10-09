package storecenter

import "context"

type StoredNoticeFact struct {
	Store      StoreSnapshot
	Connection OfficialConnectionView
}

// StoredNoticeFacts never invokes a provider, opens credentials or observes
// a new connection result. The member repository retains resource visibility.
func (r *MemberScopedStoreRepository) StoredNoticeFacts(ctx context.Context, org, after string, limit int) ([]StoredNoticeFact, string, error) {
	access, e := r.authorize(ctx, org, false)
	if e != nil {
		return nil, "", e
	}
	if limit < 1 || limit > 100 {
		return nil, "", ErrDependencyUnavailable
	}
	if after != "" {
		if _, e = canonicalUUID(after); e != nil {
			return nil, "", ErrNotFound
		}
	}
	q := r.db.WithContext(ctx).Where("organization_id=? AND deleted_at IS NULL", org)
	if !access.Administrator {
		q = q.Where("EXISTS(SELECT 1 FROM workbench_store_member_grants g WHERE g.organization_id=workbench_stores.organization_id AND g.store_id=workbench_stores.id AND g.member_id=? AND g.active=?)", access.MemberID, true)
	}
	if after != "" {
		q = q.Where("id>?", after)
	}
	var rows []workbenchStoreRecord
	if e = q.Order("id ASC").Limit(limit + 1).Find(&rows).Error; e != nil {
		return nil, "", ErrDependencyUnavailable
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = rows[len(rows)-1].ID
	}
	result := make([]StoredNoticeFact, 0, len(rows))
	for _, row := range rows {
		store, e := rehydrateRecord(row)
		if e != nil {
			return nil, "", ErrDependencyUnavailable
		}
		connection, e := r.ReadOfficialConnection(ctx, org, store.ID())
		if e != nil {
			return nil, "", e
		}
		result = append(result, StoredNoticeFact{Store: store.Snapshot(), Connection: connection})
	}
	return result, next, nil
}
