package storecenter

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"strings"
)

type MemberStoreGrantView struct {
	StoreID, MemberID string
	Active            bool
	Version           int64
}

// ReadMemberGrantReceipt returns the immutable result of the original command,
// including a no-op that preserved its version. It never reads a later grant.
func (r *MemberScopedStoreRepository) ReadMemberGrantReceipt(ctx context.Context, command MemberStoreGrantCommand) (MemberStoreGrantView, bool, error) {
	access, err := r.authorize(ctx, command.OrganizationID, true)
	if err != nil {
		return MemberStoreGrantView{}, false, err
	}
	if !access.Administrator {
		return MemberStoreGrantView{}, false, ErrNotFound
	}
	if _, err := canonicalUUID(command.OperationID); err != nil {
		return MemberStoreGrantView{}, false, ErrVersionConflict
	}
	var operation storeMemberGrantOperation
	err = r.db.WithContext(ctx).Where("organization_id=? AND operation_id=?", command.OrganizationID, command.OperationID).Take(&operation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return MemberStoreGrantView{}, false, nil
	}
	if err != nil {
		return MemberStoreGrantView{}, false, err
	}
	if operation.Fingerprint != memberGrantFingerprint(command) {
		return MemberStoreGrantView{}, false, ErrAlreadyExists
	}
	return MemberStoreGrantView{StoreID: operation.StoreID, MemberID: operation.MemberID, Active: operation.Active, Version: operation.ResultVersion}, true, nil
}

func (r *MemberScopedStoreRepository) AssignedStoreCounts(ctx context.Context, org string, members []string) (map[string]int64, error) {
	access, err := r.authorize(ctx, org, false)
	if err != nil {
		return nil, err
	}
	if len(members) > 200 {
		return nil, ErrDependencyUnavailable
	}
	counts := map[string]int64{}
	for _, member := range members {
		if _, err := validateOpaqueIdentity("member", member, MaxSubjectBytes); err != nil || (!access.Administrator && member != access.MemberID) {
			return nil, ErrNotFound
		}
		counts[member] = 0
	}
	var rows []struct {
		MemberID string
		Count    int64
	}
	err = r.db.WithContext(ctx).Table("workbench_store_member_grants g").Select("g.member_id,COUNT(*) AS count").Joins("JOIN workbench_stores s ON s.organization_id=g.organization_id AND s.id=g.store_id").Where("g.organization_id = ? AND g.member_id IN ? AND g.active = ? AND s.deleted_at IS NULL AND s.record_status <> ?", org, members, true, string(RecordStatusDeleted)).Group("g.member_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.Count < 0 {
			return nil, ErrDependencyUnavailable
		}
		counts[row.MemberID] = row.Count
	}
	return counts, nil
}
func (r *MemberScopedStoreRepository) ListMemberStores(ctx context.Context, org, member string, query StoreListQuery) (StorePage, error) {
	access, err := r.authorize(ctx, org, false)
	if err != nil {
		return StorePage{}, err
	}
	if _, err := validateOpaqueIdentity("member", member, MaxSubjectBytes); err != nil || (!access.Administrator && access.MemberID != member) {
		return StorePage{}, ErrNotFound
	}
	query.MemberID = member
	base, _ := NewGormStoreRepository(r.db)
	return base.List(ctx, org, query)
}
func (r *MemberScopedStoreRepository) ReadMemberStoreGrant(ctx context.Context, org, store, member string) (MemberStoreGrantView, error) {
	access, err := r.authorize(ctx, org, false)
	if err != nil {
		return MemberStoreGrantView{}, err
	}
	if !access.Administrator && access.MemberID != member {
		return MemberStoreGrantView{}, ErrNotFound
	}
	if strings.TrimSpace(member) == "" {
		return MemberStoreGrantView{}, ErrNotFound
	}
	if _, err := r.Get(ctx, org, store); err != nil {
		return MemberStoreGrantView{}, err
	}
	var row storeMemberGrantRow
	err = r.db.WithContext(ctx).Where("organization_id = ? AND store_id = ? AND member_id = ?", org, store, member).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return MemberStoreGrantView{StoreID: store, MemberID: member}, nil
	}
	if err != nil {
		return MemberStoreGrantView{}, err
	}
	return MemberStoreGrantView{StoreID: store, MemberID: member, Active: row.Active, Version: row.Version}, nil
}
