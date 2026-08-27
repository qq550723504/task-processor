package store

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"task-processor/internal/authidentity"
	"task-processor/internal/listingkit"
)

var imageAgentPublicationTestDatabaseCounter atomic.Uint64

func TestImageAgentPublicationTransactionExactReplayAndConflict(t *testing.T) {
	ctx, db, repository := newImageAgentPublicationStore(t)
	require.NoError(t, db.Create(&listingkit.Task{ID: "task-1", TenantID: "tenant-a", UserID: "user-a", Result: &listingkit.ListingKitResult{Country: "original"}}).Error)
	command := listingkit.ImageAgentPublicationCommit{TenantID: "tenant-a", OwnerUserID: "user-a", TaskID: "task-1", IdempotencyKey: "approve-1", Fingerprint: "fingerprint-1", Acknowledgement: listingkit.ImageAgentPublicationAcknowledgement{TaskID: "task-1", RunID: "run-1", PlanRevision: 1, ResultDigest: "digest-1", IdempotencyKey: "approve-1"}}
	var mutations atomic.Int64
	mutate := func(task *listingkit.Task) error { mutations.Add(1); task.Result.Country = "published"; return nil }

	first, err := repository.CommitImageAgentPublication(ctx, command, mutate)
	require.NoError(t, err)
	second, err := repository.CommitImageAgentPublication(ctx, command, mutate)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.EqualValues(t, 1, mutations.Load())

	conflict := command
	conflict.Fingerprint = "different-fingerprint"
	_, err = repository.CommitImageAgentPublication(ctx, conflict, mutate)
	require.ErrorIs(t, err, listingkit.ErrImageAgentPublicationConflict)
	require.EqualValues(t, 1, mutations.Load())

	wrongActor := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "tenant-a", UserID: "user-b"})
	_, err = repository.CommitImageAgentPublication(wrongActor, command, mutate)
	require.ErrorIs(t, err, listingkit.ErrImageAgentPublicationConflict)
	require.EqualValues(t, 1, mutations.Load())
}

func TestImageAgentPublicationTransactionConcurrentExactCallsConverge(t *testing.T) {
	ctx, db, repository := newImageAgentPublicationStore(t)
	require.NoError(t, db.Create(&listingkit.Task{ID: "task-1", TenantID: "tenant-a", UserID: "user-a", Result: &listingkit.ListingKitResult{Country: "original"}}).Error)
	command := listingkit.ImageAgentPublicationCommit{TenantID: "tenant-a", OwnerUserID: "user-a", TaskID: "task-1", IdempotencyKey: "approve-1", Fingerprint: "fingerprint-1", Acknowledgement: listingkit.ImageAgentPublicationAcknowledgement{TaskID: "task-1", RunID: "run-1", PlanRevision: 1, ResultDigest: "digest-1", IdempotencyKey: "approve-1"}}
	var mutations atomic.Int64
	start := make(chan struct{})
	type outcome struct {
		ack listingkit.ImageAgentPublicationAcknowledgement
		err error
	}
	results := make(chan outcome, 8)
	for range 8 {
		go func() {
			<-start
			ack, err := repository.CommitImageAgentPublication(ctx, command, func(task *listingkit.Task) error { mutations.Add(1); task.Result.Country = "published"; return nil })
			results <- outcome{ack: ack, err: err}
		}()
	}
	close(start)
	var winner listingkit.ImageAgentPublicationAcknowledgement
	for range 8 {
		result := <-results
		require.NoError(t, result.err)
		if winner.TaskID == "" {
			winner = result.ack
		}
		require.Equal(t, winner, result.ack)
	}
	require.EqualValues(t, 1, mutations.Load())
}

func newImageAgentPublicationStore(t *testing.T) (context.Context, *gorm.DB, listingkit.ImageAgentPublicationTransactionRepository) {
	t.Helper()
	dsn := fmt.Sprintf("file:image-agent-publication-store-%s-%d?mode=memory&cache=shared", t.Name(), imageAgentPublicationTestDatabaseCounter.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	require.NoError(t, db.AutoMigrate(&listingkit.Task{}))
	require.NoError(t, AutoMigrateImageAgentPublicationReceipts(db))
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "tenant-a", UserID: "user-a"})
	return ctx, db, NewImageAgentPublicationTransactionRepository(db)
}
