package orgresourceadapter

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"gorm.io/gorm"
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

type testSQLStateError struct {
	state string
}

func (err testSQLStateError) Error() string    { return "postgres state " + err.state }
func (err testSQLStateError) SQLState() string { return err.state }
