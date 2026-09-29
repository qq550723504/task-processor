package orgresourceadapter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"task-processor/internal/ledger/orgresource"
)

// This bounded native read includes departed members' positions; reads never
// reclaim or remove holdings. Current authorization belongs to the application.
func (r *GormMemberAllocationRepository) ListPositions(ctx context.Context, org string) ([]orgresource.MemberResourcePosition, error) {
	if !orgresource.ValidMemberResourcePositionIdentity(org, "bounded-reader", orgresource.ResourceDataRow) {
		return nil, orgresource.ErrInvalidInput
	}
	var rows []memberResourcePositionRow
	if err := r.db.WithContext(ctx).Where("organization_id = ?", org).Order("member_id,resource_type").Limit(201).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) > 200 {
		return nil, orgresource.ErrBalanceUnavailable
	}
	result := make([]orgresource.MemberResourcePosition, 0, len(rows))
	for _, row := range rows {
		if !orgresource.IsMemberAllocatedResource(orgresource.ResourceType(row.ResourceType)) || row.Free < 0 || row.Reserved < 0 || row.Consumed < 0 || row.Version <= 0 {
			return nil, orgresource.ErrBalanceUnavailable
		}
		result = append(result, memberResourcePosition(row))
	}
	return result, nil
}

// Read the immutable matching result before quote expiry/current position checks.
// It admits no mutation and never trusts a caller-provided success receipt.
func (r *GormMemberAllocationRepository) ReadTransferReplay(ctx context.Context, c orgresource.MemberResourceTransfer) (orgresource.MemberResourceTransferResult, bool, error) {
	if !orgresource.ValidMemberResourceTransfer(c) {
		return orgresource.MemberResourceTransferResult{}, false, orgresource.ErrInvalidInput
	}
	encoded, _ := json.Marshal(c)
	sum := sha256.Sum256(encoded)
	var operation organizationResourceOperationRow
	err := r.db.WithContext(ctx).Where("organization_id = ? AND operation_id = ?", c.OrganizationID, c.OperationID).Take(&operation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return orgresource.MemberResourceTransferResult{}, false, nil
	}
	if err != nil {
		return orgresource.MemberResourceTransferResult{}, false, err
	}
	if operation.OperationType != string(c.Action) || operation.RequestFingerprint != hex.EncodeToString(sum[:]) || operation.State != "succeeded" {
		return orgresource.MemberResourceTransferResult{}, false, orgresource.ErrIdempotencyKeyConflict
	}
	var result orgresource.MemberResourceTransferResult
	if err := json.Unmarshal([]byte(operation.ImmutableResult), &result); err != nil || result.Position.OrganizationID != c.OrganizationID || result.Position.MemberID != c.MemberID || result.Position.ResourceType != c.ResourceType {
		return result, false, orgresource.ErrBalanceUnavailable
	}
	result.Replayed = true
	return result, true, nil
}
