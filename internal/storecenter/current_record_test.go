package storecenter_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"task-processor/internal/storecenter"
)

func TestCurrentStoreRecordDisableEnablePreservesPaidService(t *testing.T) {
	db := openStoreDB(t)
	repo, err := storecenter.NewGormStoreRepository(db)
	require.NoError(t, err)
	store := newPersistenceStore(t, "org-a", "00000000-0000-4000-8000-000000000971", "00000000-0000-4000-8000-000000000972", "00000000-0000-4000-8000-000000000973", "Current store", "US", "current-971", testPersistenceTime)
	_, _, err = repo.CreateOrReplay(context.Background(), "org-a", store)
	require.NoError(t, err)
	require.NoError(t, store.TransitionTo(storecenter.RecordStatusActive, "actor", testPersistenceTime.Add(time.Minute)))
	require.NoError(t, repo.Save(context.Background(), "org-a", store, 1))
	start, expiry := testPersistenceTime, testPersistenceTime.Add(30*24*time.Hour)
	require.NoError(t, db.Table("workbench_stores").Where("id = ?", store.ID()).Updates(map[string]any{"service_status": "active", "service_started_at": start, "service_expires_at": expiry}).Error)
	store, err = repo.Get(context.Background(), "org-a", store.ID())
	require.NoError(t, err)
	require.NoError(t, store.TransitionTo(storecenter.RecordStatusDisabled, "actor", testPersistenceTime.Add(2*time.Minute)))
	require.NoError(t, repo.Save(context.Background(), "org-a", store, 2))
	var service struct {
		Status string    `gorm:"column:service_status"`
		Start  time.Time `gorm:"column:service_started_at"`
		Expiry time.Time `gorm:"column:service_expires_at"`
	}
	require.NoError(t, db.Table("workbench_stores").Where("id = ?", store.ID()).Take(&service).Error)
	require.Equal(t, "active", service.Status, "record disable must not suspend a paid service")
	require.True(t, service.Start.Equal(start))
	require.True(t, service.Expiry.Equal(expiry))

	require.ErrorIs(t, db.Transaction(func(tx *gorm.DB) error {
		return repo.ApplyServiceState(context.Background(), tx, storecenter.ServiceStoreMutation{
			Identity: storecenter.ServiceStoreIdentity{OrganizationID: "org-a", StoreID: store.ID()}, ExpectedVersion: 3, ActorSubject: "actor", OccurredAt: testPersistenceTime.Add(3 * time.Minute), State: storecenter.StoreServiceState{RecordStatus: storecenter.RecordStatusActive, ServiceStatus: storecenter.ServiceStatusActive, StartedAt: &start, ExpiresAt: &expiry},
		})
	}), storecenter.ErrInvalidServiceTransition, "prepared service execution cannot re-enable a disabled record")
	store, err = repo.Get(context.Background(), "org-a", store.ID())
	require.NoError(t, err)
	require.NoError(t, store.TransitionTo(storecenter.RecordStatusActive, "actor", testPersistenceTime.Add(3*time.Minute)))
	require.NoError(t, repo.Save(context.Background(), "org-a", store, 3), "record enable must not require service reactivation")
	require.NoError(t, db.Table("workbench_stores").Where("id = ?", store.ID()).Take(&service).Error)
	require.Equal(t, "active", service.Status)
	require.True(t, service.Expiry.Equal(expiry))
}

func TestCurrentStoreSchemaHasOneRecordStatus(t *testing.T) {
	db := openStoreDB(t)
	require.True(t, db.Migrator().HasColumn("workbench_stores", "record_status"))
	require.False(t, db.Migrator().HasColumn("workbench_stores", "lifecycle_status"))
	require.False(t, db.Migrator().HasColumn("workbench_stores", "service_history_resolution_status"))
}

func TestCurrentDeletedStoreRequiresDeletionFact(t *testing.T) {
	store := newTestStore(t)
	snapshot := store.Snapshot()
	snapshot.RecordStatus = storecenter.RecordStatusDeleted
	snapshot.Version = 4
	snapshot.DeleteOperationKey = testIdempotencyKey
	snapshot.DeletedAt = nil
	_, err := storecenter.RehydrateStore(snapshot)
	require.Error(t, err)
}
