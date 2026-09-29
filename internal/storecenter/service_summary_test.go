package storecenter_test

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"task-processor/internal/storecenter"
	"testing"
	"time"
)

func TestNativeServiceSummaryScopesDatesAndExcludesUnavailableRecords(t *testing.T) {
	db := openStoreDB(t)
	repo, err := storecenter.NewGormStoreRepository(db)
	require.NoError(t, err)
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	summary, err := repo.ReadServiceSummary(context.Background(), "org", now)
	require.NoError(t, err)
	require.Equal(t, storecenter.ServiceSummary{}, summary)
	cases := []struct {
		org, record, service string
		start, expiry        time.Time
	}{
		{"org", "active", "pending_activation", time.Time{}, time.Time{}},
		{"org", "active", "active", now.Add(-time.Hour), now.Add(8 * 24 * time.Hour)},
		{"org", "active", "active", now.Add(-time.Hour), now.Add(7 * 24 * time.Hour)},
		{"org", "active", "active", now, now.Add(time.Second)},
		{"org", "active", "active", now.Add(-time.Hour), now},
		{"org", "active", "expired", now.Add(-time.Hour), now.Add(-time.Second)},
		{"org", "disabled", "active", now.Add(-time.Hour), now.Add(time.Hour)},
		{"org", "active", "suspended", now.Add(-time.Hour), now.Add(time.Hour)},
		{"org", "active", "active", now.Add(time.Hour), now.Add(2 * time.Hour)},
		{"org", "deleting", "", time.Time{}, time.Time{}},
		{"org", "deleted", "", time.Time{}, time.Time{}},
		{"other", "active", "active", now.Add(-time.Hour), now.Add(time.Hour)},
	}
	for _, test := range cases {
		id := uuid.NewString()
		value, err := storecenter.NewStore(storecenter.CreateStoreInput{ID: id, OrganizationID: test.org, ActorSubject: "creator", Name: "Synthetic", Platform: "shein", Region: "SG", ExternalStoreID: id, CreateIdempotencyKey: uuid.NewString(), OccurredAt: now.Add(-24 * time.Hour)})
		require.NoError(t, err)
		_, _, err = repo.CreateOrReplay(context.Background(), test.org, value)
		require.NoError(t, err)
		changes := map[string]any{"record_status": test.record, "service_status": test.service, "version": 3}
		if !test.start.IsZero() {
			changes["service_started_at"] = test.start
			changes["service_expires_at"] = test.expiry
		}
		if test.record == "deleting" || test.record == "deleted" {
			changes["service_status"] = nil
			changes["delete_operation_key"] = uuid.NewString()
		}
		if test.record == "deleted" {
			changes["deleted_at"] = now
		}
		require.NoError(t, db.Table("workbench_stores").Where("id = ?", id).Updates(changes).Error)
	}
	summary, err = repo.ReadServiceSummary(context.Background(), "org", now)
	require.NoError(t, err)
	require.Equal(t, storecenter.ServiceSummary{Records: 9, Active: 3, Expired: 2, ExpiringSoon: 2}, summary)
}

func TestNativeStoreCreateRejectsPaidServiceSnapshot(t *testing.T) {
	repo := newStoreRepository(t)
	value := nativeCandidate(t, uuid.NewString(), "creator")
	snapshot := value.Snapshot()
	start, expiry := snapshot.CreatedAt, snapshot.CreatedAt.Add(30*24*time.Hour)
	snapshot.ServiceStatus = storecenter.ServiceStatusActive
	snapshot.ServiceStartedAt = &start
	snapshot.ServiceExpiresAt = &expiry
	paid, err := storecenter.RehydrateStore(snapshot)
	require.NoError(t, err)
	_, _, err = repo.CreateOrReplay(context.Background(), "org-a", paid)
	require.Error(t, err)
}
