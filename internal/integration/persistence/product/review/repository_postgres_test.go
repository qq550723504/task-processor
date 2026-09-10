package reviewpersistence

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/product/review"
	"task-processor/internal/product/sourcing"
)

type transactionSourceReaderStub struct{}

func (transactionSourceReaderStub) Read(context.Context, string) (sourcing.PersistedPublication, error) {
	return sourcing.PersistedPublication{}, errors.New("unused source reader")
}

type countingTransactionSourceReader struct{ reads *atomic.Int32 }

func (reader countingTransactionSourceReader) Read(context.Context, string) (sourcing.PersistedPublication, error) {
	reader.reads.Add(1)
	return sourcing.PersistedPublication{}, nil
}

func postgresReviewRepositoryFixture(t *testing.T) (*Repository, *gorm.DB) {
	t.Helper()
	dsn := os.Getenv("ISSUE382_TEST_DSN")
	if dsn == "" {
		t.Skip("requires isolated PostgreSQL ISSUE382_TEST_DSN")
	}
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	root, err := gorm.Open(postgres.Open(dsn), cfg)
	require.NoError(t, err)
	schema := "review_repository_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, root.Exec("CREATE SCHEMA "+schema).Error)
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		raw, _ := db.DB()
		_ = raw.Close()
		_ = root.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		raw, _ = root.DB()
		_ = raw.Close()
	})
	require.NoError(t, InstallSchema(db))
	repository, err := NewRepository(db, func(*gorm.DB) (review.SourcePublicationReader, error) {
		return transactionSourceReaderStub{}, nil
	})
	require.NoError(t, err)
	return repository, db
}

type operationResult struct {
	view review.View
	err  error
}

func TestPostgresRunUsesReadCommitted(t *testing.T) {
	repository, _ := postgresReviewRepositoryFixture(t)
	originalFactory := repository.sourceReaderFactory
	var isolation string
	repository.sourceReaderFactory = func(tx *gorm.DB) (review.SourcePublicationReader, error) {
		if err := tx.Raw("SHOW transaction_isolation").Scan(&isolation).Error; err != nil {
			return nil, err
		}
		return originalFactory(tx)
	}
	op := review.Operation{Scope: review.Scope{Org: "org", Actor: "actor"}, Key: "isolation", Fingerprint: "payload"}
	want := review.View{ID: "00000000-0000-0000-0000-000000000003", Owner: "actor", State: "pending", Revision: 1}
	got, err := repository.Run(context.Background(), op, func(tx review.Tx) (review.View, error) {
		return want, tx.Complete(want)
	})
	require.NoError(t, err)
	require.Equal(t, want, got)
	require.Equal(t, "read committed", isolation)
}

func TestPostgresOperationWaiterReplaysCommittedReceipt(t *testing.T) {
	repository, db := postgresReviewRepositoryFixture(t)
	var sourceReads atomic.Int32
	repository.sourceReaderFactory = func(*gorm.DB) (review.SourcePublicationReader, error) {
		return countingTransactionSourceReader{reads: &sourceReads}, nil
	}
	var lockAttempts atomic.Int32
	secondAtLock := make(chan struct{})
	repository.beforeOperationLock = func() {
		if lockAttempts.Add(1) == 2 {
			close(secondAtLock)
		}
	}

	op := review.Operation{Scope: review.Scope{Org: "org", Actor: "actor"}, Key: "same-key", Fingerprint: "same-payload"}
	for range 2 {
		_, found, err := repository.Preflight(context.Background(), op)
		require.NoError(t, err)
		require.False(t, found, "both requests may miss preflight before the mutation race")
	}
	want := review.View{ID: "00000000-0000-0000-0000-000000000001", Owner: "actor", State: "pending", Revision: 1}
	firstInside := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan operationResult, 1)
	secondDone := make(chan operationResult, 1)
	var mutations atomic.Int32

	run := func(block bool, done chan<- operationResult) {
		view, err := repository.Run(context.Background(), op, func(tx review.Tx) (review.View, error) {
			if replayed, found, replayErr := tx.Replay(); replayErr != nil || found {
				return replayed, replayErr
			}
			_, _ = tx.SourceReader().Read(context.Background(), "proof-consumption")
			mutations.Add(1)
			if block {
				close(firstInside)
				<-releaseFirst
			}
			return want, tx.Complete(want)
		})
		done <- operationResult{view: view, err: err}
	}

	go run(true, firstDone)
	<-firstInside
	go run(false, secondDone)
	<-secondAtLock
	select {
	case result := <-secondDone:
		t.Fatalf("waiter passed the operation lock before the first commit: %+v", result)
	default:
	}
	close(releaseFirst)

	first := <-firstDone
	second := <-secondDone
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.Equal(t, want, first.view)
	require.Equal(t, want, second.view)
	require.Equal(t, int32(1), mutations.Load())
	require.Equal(t, int32(1), sourceReads.Load(), "the race replay must return before consuming source proof")
	var operations int64
	require.NoError(t, db.Table("product_title_operations").Count(&operations).Error)
	require.Equal(t, int64(1), operations)
}

func TestPostgresPreflightReturnsReplayOrConflictWithoutBusinessWrites(t *testing.T) {
	repository, db := postgresReviewRepositoryFixture(t)
	op := review.Operation{Scope: review.Scope{Org: "org", Actor: "actor"}, Key: "preflight", Fingerprint: "payload-a"}
	want := review.View{ID: "00000000-0000-0000-0000-000000000004", Owner: "actor", State: "pending", Revision: 1}
	_, err := repository.Run(context.Background(), op, func(tx review.Tx) (review.View, error) {
		return want, tx.Complete(want)
	})
	require.NoError(t, err)

	sourceFactories := atomic.Int32{}
	repository.sourceReaderFactory = func(*gorm.DB) (review.SourcePublicationReader, error) {
		sourceFactories.Add(1)
		return transactionSourceReaderStub{}, nil
	}
	replayed, found, err := repository.Preflight(context.Background(), op)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, want, replayed)
	_, found, err = repository.Preflight(context.Background(), review.Operation{Scope: op.Scope, Key: op.Key, Fingerprint: "payload-b"})
	require.ErrorIs(t, err, review.ErrConflict)
	require.False(t, found)
	require.Zero(t, sourceFactories.Load(), "preflight must not construct the transaction source reader")

	var proposals, operations int64
	require.NoError(t, db.Table("product_title_proposals").Count(&proposals).Error)
	require.NoError(t, db.Table("product_title_operations").Count(&operations).Error)
	require.Zero(t, proposals)
	require.Equal(t, int64(1), operations)
	raw, err := db.DB()
	require.NoError(t, err)
	require.Zero(t, raw.Stats().InUse)
	var advisoryLocks int64
	require.NoError(t, db.Raw("SELECT count(*) FROM pg_locks WHERE locktype = 'advisory' AND database = (SELECT oid FROM pg_database WHERE datname = current_database())").Scan(&advisoryLocks).Error)
	require.Zero(t, advisoryLocks)
}

func TestPostgresOperationWaiterConflictsOnDifferentPayload(t *testing.T) {
	repository, _ := postgresReviewRepositoryFixture(t)
	var lockAttempts atomic.Int32
	secondAtLock := make(chan struct{})
	repository.beforeOperationLock = func() {
		if lockAttempts.Add(1) == 2 {
			close(secondAtLock)
		}
	}

	scope := review.Scope{Org: "org", Actor: "actor"}
	firstOperation := review.Operation{Scope: scope, Key: "same-key", Fingerprint: "payload-a"}
	secondOperation := review.Operation{Scope: scope, Key: "same-key", Fingerprint: "payload-b"}
	want := review.View{ID: "00000000-0000-0000-0000-000000000002", Owner: "actor", State: "pending", Revision: 1}
	firstInside := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	secondDone := make(chan error, 1)
	var mutations atomic.Int32

	go func() {
		_, err := repository.Run(context.Background(), firstOperation, func(tx review.Tx) (review.View, error) {
			if replayed, found, replayErr := tx.Replay(); replayErr != nil || found {
				return replayed, replayErr
			}
			mutations.Add(1)
			close(firstInside)
			<-releaseFirst
			return want, tx.Complete(want)
		})
		firstDone <- err
	}()
	<-firstInside
	go func() {
		_, err := repository.Run(context.Background(), secondOperation, func(tx review.Tx) (review.View, error) {
			if replayed, found, replayErr := tx.Replay(); replayErr != nil || found {
				return replayed, replayErr
			}
			mutations.Add(1)
			return want, tx.Complete(want)
		})
		secondDone <- err
	}()
	<-secondAtLock
	close(releaseFirst)

	require.NoError(t, <-firstDone)
	require.ErrorIs(t, <-secondDone, review.ErrConflict)
	require.Equal(t, int32(1), mutations.Load())
}
