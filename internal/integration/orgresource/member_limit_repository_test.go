package orgresourceadapter

import (
	"context"
	"task-processor/internal/ledger/orgresource"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMemberLimitSnapshotCanonicalizesDatabaseMonthToUTC(t *testing.T) {
	month := time.Date(2026, 10, 1, 0, 0, 0, 0, time.FixedZone("UTC", 0))
	wanted := orgresource.AIPointMonthStart(month)
	require.False(t, month == wanted, "database UTC location differs from time.UTC")
	read := limitSnapshot(memberAIPointLimitRow{OrganizationID: "org-1", MemberID: "member-1", MonthlyLimit: 120, Version: 1}, memberAIPointMonthRow{MonthStart: month, Reserved: 3, Consumed: 6})
	require.True(t, read.MonthStart == wanted, "the generation contract requires the canonical UTC month")
	require.EqualValues(t, 120, read.MonthlyLimit)
	require.EqualValues(t, 3, read.Reserved)
	require.EqualValues(t, 6, read.Consumed)
}

func TestMemberAIPointLimitOriginalOperationSurvivesMonthRollover(t *testing.T) {
	db := openSQLiteStore(t)
	require.NoError(t, AutoMigrate(db))
	repository, err := NewGormMemberLimitRepository(db, TransactionConfig{})
	require.NoError(t, err)
	now := time.Date(2026, 9, 30, 23, 59, 0, 0, time.UTC)
	repository.now = func() time.Time { return now }
	input := orgresource.SetMemberLimitExecution{OrganizationID: "org-1", MemberID: "member-1", ActorID: "admin-1", OperationID: "limit-1", Target: 20}
	first, err := repository.SetMonthlyLimit(context.Background(), input)
	require.NoError(t, err)
	require.EqualValues(t, 1, first.Version)
	require.Equal(t, orgresource.AIPointMonthStart(now), first.MonthStart)
	now = now.Add(2 * time.Minute)
	replay, err := repository.SetMonthlyLimit(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, first, replay)
	read, err := repository.ReadMonthlyLimit(context.Background(), input.OrganizationID, input.MemberID)
	require.NoError(t, err)
	require.EqualValues(t, 20, read.MonthlyLimit)
	require.EqualValues(t, 1, read.Version)
	require.Equal(t, orgresource.AIPointMonthStart(now), read.MonthStart)
	changed := input
	changed.Target = 21
	_, err = repository.SetMonthlyLimit(context.Background(), changed)
	require.ErrorIs(t, err, orgresource.ErrIdempotencyKeyConflict)
	changed = input
	changed.MemberID = "member-2"
	_, err = repository.SetMonthlyLimit(context.Background(), changed)
	require.ErrorIs(t, err, orgresource.ErrIdempotencyKeyConflict)
	_, err = repository.ReadMonthlyLimit(context.Background(), "other-org", input.MemberID)
	require.ErrorIs(t, err, orgresource.ErrMemberLimitUnavailable)
	var buckets int64
	require.NoError(t, db.Model(&organizationResourceBucketRow{}).Count(&buckets).Error)
	require.Zero(t, buckets, "setting a limit must never mint enterprise resources")
}
