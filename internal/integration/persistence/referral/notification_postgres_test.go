//go:build integration

package referral

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNoticeReadsUsePrivateNativeKeysets(t *testing.T) {
	owner, runtime := ownedDatabase(t)
	r, err := New(runtime)
	require.NoError(t, err)
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Microsecond)
	for i := range 121 {
		for _, subject := range []string{"self", "other"} {
			id := uuid.NewString()
			require.NoError(t, owner.Create(&earningLedgerEntry{EntryID: id, Referrer: subject, Currency: "CNY", PaymentID: id, EntryType: "REFUND_ADJUSTMENT", AmountMinor: -1, ReferenceID: id, OccurredAt: at.Add(time.Duration(i) * time.Second)}).Error)
			require.NoError(t, owner.Create(&withdrawalRow{ID: id, Referrer: subject, PayoutMethodID: "private-method", Currency: "CNY", Method: "ALIPAY", AmountMinor: 10000, Status: "REQUESTED", Version: 1, CreatedAt: at.Add(time.Duration(i) * time.Second), UpdatedAt: at}).Error)
		}
	}
	// A normal commission is not an adjustment notice.
	id := uuid.NewString()
	require.NoError(t, owner.Create(&earningLedgerEntry{EntryID: id, Referrer: "self", Currency: "CNY", PaymentID: id, EntryType: "COMMISSION", AmountMinor: 1, ReferenceID: id, OccurredAt: at}).Error)
	adjustments, cursor, err := r.ListNoticeAdjustments(ctx, "self", "", 100)
	require.NoError(t, err)
	require.Len(t, adjustments, 100)
	require.NotEmpty(t, cursor)
	tail, next, err := r.ListNoticeAdjustments(ctx, "self", cursor, 100)
	require.NoError(t, err)
	require.Len(t, tail, 21)
	require.Empty(t, next)
	seen := map[string]bool{}
	for _, row := range append(adjustments, tail...) {
		require.Equal(t, "self", row.Referrer)
		require.Equal(t, "REFUND_ADJUSTMENT", row.EntryType)
		require.False(t, seen[row.EntryID])
		seen[row.EntryID] = true
	}
	_, _, err = r.ListNoticeAdjustments(ctx, "other", cursor, 100)
	require.Error(t, err, "foreign subject cannot supply the cursor fact")
	withdrawals, cursor, err := r.ListNoticeWithdrawals(ctx, "self", "", 100)
	require.NoError(t, err)
	require.Len(t, withdrawals, 100)
	tailWithdrawals, next, err := r.ListNoticeWithdrawals(ctx, "self", cursor, 100)
	require.NoError(t, err)
	require.Len(t, tailWithdrawals, 21)
	require.Empty(t, next)
	seen = map[string]bool{}
	for _, row := range append(withdrawals, tailWithdrawals...) {
		require.Equal(t, "self", row.Referrer)
		require.False(t, seen[row.ID])
		seen[row.ID] = true
	}
	_, _, err = r.ListNoticeWithdrawals(ctx, "other", cursor, 100)
	require.Error(t, err)
}
