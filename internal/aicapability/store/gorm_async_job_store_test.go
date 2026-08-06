package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"task-processor/internal/aicapability"
)

func TestGormAsyncJobBindingRoundTripAndIdempotency(t *testing.T) {
	db := newAsyncJobBindingDB(t)
	store := NewGormAsyncJobBindingStore(db)
	binding := aicapability.AsyncJobBinding{
		JobID: " job-1 ", TenantID: " tenant-1 ", UserID: " user-1 ", Capability: aicapability.CapabilityListingKitStudioImage,
		Operation: aicapability.OperationAsyncImageGenerate, ProviderID: " provider-a ", ModelID: " model-a ", RoutingKey: " route-a ",
		CredentialReference: " credential-a ", PolicyVersion: " policy-v1 ", ConfigurationVersion: " config-v1 ",
		SubmittedAt: time.Date(2026, 8, 6, 1, 2, 3, 0, time.UTC), Status: " queued ",
	}
	require.NoError(t, store.PutAsyncJobBinding(context.Background(), binding))
	require.NoError(t, store.PutAsyncJobBinding(context.Background(), binding))

	got, err := store.GetAsyncJobBinding(context.Background(), " job-1 ")
	require.NoError(t, err)
	require.Equal(t, "job-1", got.JobID)
	require.Equal(t, "tenant-1", got.TenantID)
	require.Equal(t, "provider-a", got.ProviderID)
	require.Equal(t, "route-a", got.RoutingKey)
	require.Equal(t, "queued", got.Status)
}

func TestGormAsyncJobBindingRejectsConflictingRouteWithoutOverwrite(t *testing.T) {
	db := newAsyncJobBindingDB(t)
	store := NewGormAsyncJobBindingStore(db)
	binding := validAsyncJobBinding()
	require.NoError(t, store.PutAsyncJobBinding(context.Background(), binding))

	conflict := binding
	conflict.RoutingKey = "route-b"
	err := store.PutAsyncJobBinding(context.Background(), conflict)
	require.True(t, errors.Is(err, aicapability.ErrAsyncJobBindingConflict), "error = %v", err)

	got, err := store.GetAsyncJobBinding(context.Background(), binding.JobID)
	require.NoError(t, err)
	require.Equal(t, "route-a", got.RoutingKey)
}

func TestGormAsyncJobBindingLookupStatusAndSensitiveColumns(t *testing.T) {
	db := newAsyncJobBindingDB(t)
	store := NewGormAsyncJobBindingStore(db)
	binding := validAsyncJobBinding()
	require.NoError(t, store.PutAsyncJobBinding(context.Background(), binding))
	require.NoError(t, store.UpdateAsyncJobBindingStatus(context.Background(), binding.JobID, "failed", aicapability.ErrorProviderRejected))

	got, err := store.GetAsyncJobBinding(context.Background(), binding.JobID)
	require.NoError(t, err)
	require.Equal(t, "failed", got.Status)
	require.Equal(t, aicapability.ErrorProviderRejected, got.LastErrorCategory)

	_, err = store.GetAsyncJobBinding(context.Background(), "missing")
	require.ErrorIs(t, err, aicapability.ErrAsyncJobBindingNotFound)

	columns, err := db.Migrator().ColumnTypes(&asyncJobRow{})
	require.NoError(t, err)
	for _, column := range columns {
		switch column.Name() {
		case "prompt", "raw_prompt", "response", "raw_response", "image_bytes", "api_key", "cookie", "authorization":
			t.Fatalf("sensitive column %q must not exist", column.Name())
		}
	}
}

func TestGormAsyncJobBindingRejectsInvalidBinding(t *testing.T) {
	db := newAsyncJobBindingDB(t)
	store := NewGormAsyncJobBindingStore(db)
	require.ErrorIs(t, store.PutAsyncJobBinding(context.Background(), aicapability.AsyncJobBinding{}), aicapability.ErrAsyncJobBindingInvalid)
	require.ErrorIs(t, store.UpdateAsyncJobBindingStatus(context.Background(), "missing", "failed", ""), aicapability.ErrAsyncJobBindingNotFound)
}

func validAsyncJobBinding() aicapability.AsyncJobBinding {
	return aicapability.AsyncJobBinding{
		JobID: "job-1", TenantID: "tenant-1", Capability: aicapability.CapabilityListingKitStudioImage,
		Operation: aicapability.OperationAsyncImageGenerate, ProviderID: "provider-a", ModelID: "model-a", RoutingKey: "route-a",
		SubmittedAt: time.Date(2026, 8, 6, 1, 2, 3, 0, time.UTC), Status: "queued",
	}
}

func newAsyncJobBindingDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newInvocationLedgerDB(t)
	require.NoError(t, AutoMigrateAsyncJobBindings(db))
	return db
}
