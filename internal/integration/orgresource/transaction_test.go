package orgresourceadapter

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/ledger/orgresource"
)

func TestPostgresRetryClassifierUsesOnlyBoundedConcurrencyStates(t *testing.T) {
	runner := &transactionRunner{dialect: "postgres"}
	for _, state := range []string{"40001", "40P01", "55P03", "57014"} {
		if !runner.retryable(fmt.Errorf("wrapped: %w", testSQLStateError{state: state})) {
			t.Fatalf("SQLSTATE %s was not retryable", state)
		}
	}
	for _, state := range []string{"23505", "42501", "42P01"} {
		if runner.retryable(testSQLStateError{state: state}) {
			t.Fatalf("SQLSTATE %s was unexpectedly retryable", state)
		}
	}
}

func TestTransactionRunnerStopsOnCallerCancellationBeforeDatabaseWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runner := &transactionRunner{config: TransactionConfig{}.withDefaults()}
	called := false
	err := runner.run(ctx, func(_ *gorm.DB) error {
		called = true
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("run() error = %v, want context.Canceled", err)
	}
	if called {
		t.Fatal("operation ran after caller cancellation")
	}
}

func TestTransactionRunnerRetriesTransientSQLiteReads(t *testing.T) {
	runner := &transactionRunner{dialect: "sqlite", config: TransactionConfig{
		MaxAttempts: 3, TotalRetryBudget: time.Second, TransactionTimeout: time.Second, BaseRetryDelay: time.Millisecond,
	}.withDefaults()}
	attempts := 0
	err := runner.runRead(context.Background(), func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("database is locked (5) (SQLITE_BUSY)")
		}
		return nil
	})
	if err != nil || attempts != 3 {
		t.Fatalf("runRead() = %v after %d attempts, want success after 3", err, attempts)
	}
}

func TestTransactionRunnerBoundsTransientReadRetries(t *testing.T) {
	runner := &transactionRunner{dialect: "sqlite", config: TransactionConfig{
		MaxAttempts: 2, TotalRetryBudget: time.Second, TransactionTimeout: time.Second, BaseRetryDelay: time.Millisecond,
	}.withDefaults()}
	attempts := 0
	err := runner.runRead(context.Background(), func(context.Context) error {
		attempts++
		return errors.New("database is locked (5) (SQLITE_BUSY)")
	})
	if !errors.Is(err, orgresource.ErrConcurrencyRetry) || attempts != 2 {
		t.Fatalf("runRead() = %v after %d attempts, want bounded concurrency retry", err, attempts)
	}
}

func TestTransactionRunnerRetriesRunnerOwnedDeadlineAsConcurrency(t *testing.T) {
	runner := &transactionRunner{dialect: "postgres", config: TransactionConfig{
		MaxAttempts: 2, TotalRetryBudget: time.Second, TransactionTimeout: 5 * time.Millisecond, BaseRetryDelay: time.Millisecond,
	}.withDefaults()}
	attempts := 0
	err := runner.runRead(context.Background(), func(ctx context.Context) error {
		attempts++
		<-ctx.Done()
		return ctx.Err()
	})
	if !errors.Is(err, orgresource.ErrConcurrencyRetry) || attempts != 2 {
		t.Fatalf("runRead() = %v after %d attempts, want bounded deadline retries", err, attempts)
	}
}

type testSQLStateError struct {
	state string
}

func (err testSQLStateError) Error() string    { return "postgres state " + err.state }
func (err testSQLStateError) SQLState() string { return err.state }
