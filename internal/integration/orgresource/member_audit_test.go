package orgresourceadapter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/ledger/orgresource"
)

func TestMemberAuditReadsCommittedOriginalFactsWithScopedStablePages(t *testing.T) {
	db := openSQLiteStore(t)
	require.NoError(t, AutoMigrate(db))
	ctx := context.Background()
	transfers, err := NewGormMemberAllocationRepository(db, TransactionConfig{})
	require.NoError(t, err)
	limits, err := NewGormMemberLimitRepository(db, TransactionConfig{})
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Microsecond)
	transfers.now, limits.now = func() time.Time { return now }, func() time.Time { return now }
	for _, org := range []string{"org-a", "org-b"} {
		require.NoError(t, db.Create(&organizationResourceBucketRow{OrganizationID: org, ResourceType: "data_row", Available: 100}).Error)
		command := orgresource.MemberResourceTransfer{OrganizationID: org, MemberID: "member", ActorID: "admin", OperationID: "allocate", ResourceType: orgresource.ResourceDataRow, Action: orgresource.MemberResourceAllocate, Quantity: 10}
		_, err = transfers.Transfer(ctx, command)
		require.NoError(t, err)
		_, err = transfers.Transfer(ctx, command) // Original replay adds no audit.
		require.NoError(t, err)
		command.Action, command.OperationID, command.Quantity, command.ExpectedVersion = orgresource.MemberResourceReclaim, "reclaim", 3, 1
		_, err = transfers.Transfer(ctx, command)
		require.NoError(t, err)
		failed := command
		failed.Action, failed.OperationID, failed.Quantity, failed.ExpectedVersion = orgresource.MemberResourceAllocate, "failed-allocate", 1000, 2
		_, err = transfers.Transfer(ctx, failed)
		require.ErrorIs(t, err, orgresource.ErrInsufficientBalance)
		_, err = limits.SetMonthlyLimit(ctx, orgresource.SetMemberLimitExecution{OrganizationID: org, MemberID: "member", ActorID: "other-admin", OperationID: "limit", Target: 1000})
		require.NoError(t, err)
		_, err = limits.SetMonthlyLimit(ctx, orgresource.SetMemberLimitExecution{OrganizationID: org, MemberID: "member", ActorID: "admin", OperationID: "zero-limit", Target: 0, ExpectedVersion: 1})
		require.NoError(t, err)
	}
	reader, err := NewGormRepository(db, TransactionConfig{})
	require.NoError(t, err)
	var cursor *orgresource.MemberAuditPosition
	var seen []string
	var amounts []int64
	for {
		page, err := reader.ListMemberAudit(ctx, "org-a", 1, "", "", cursor)
		require.NoError(t, err)
		require.Len(t, page.Items, 1)
		item := page.Items[0]
		require.Equal(t, "org-a", item.OrganizationID)
		seen, amounts = append(seen, item.OperationID), append(amounts, item.Quantity)
		if page.Next == nil {
			break
		}
		cursor = page.Next
		require.Less(t, len(seen), 5)
	}
	require.Equal(t, []string{"zero-limit", "limit", "reclaim", "allocate"}, seen)
	require.Equal(t, []int64{0, 1000, 3, 10}, amounts)
	page, err := reader.ListMemberAudit(ctx, "org-a", 10, "other-admin", "set_member_ai_point_limit", nil)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, int64(1000), page.Items[0].Quantity) // Current cap is now zero.
	page, err = reader.ListMemberAudit(ctx, "org-a", 10, "admin", "reclaim_member_resource", nil)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, int64(2), page.Items[0].Version)
	// A corrupt original receipt must fail explicitly, not become an empty page
	// or derive its member/value from mutable current state.
	require.NoError(t, db.Model(&organizationResourceAuditLogRow{}).Where("organization_id=? AND operation_id=?", "org-a", "limit").Update("payload", "{}").Error)
	_, err = reader.ListMemberAudit(ctx, "org-a", 10, "other-admin", "", nil)
	require.Error(t, err)
}
