package storecenter

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestStoreHistoryRetryClassifierAllowsOnlyTransientConcurrencyFailures(t *testing.T) {
	for _, state := range []string{"40001", "40P01", "55P03"} {
		if !isStoreHistoryTransientConcurrencyError(fmt.Errorf("wrapped: %w", storeHistorySQLStateError{state: state})) {
			t.Fatalf("SQLSTATE %s was not retryable", state)
		}
	}
	for _, err := range []error{
		errors.New("database is locked (5) (SQLITE_BUSY)"),
		errors.New("database table is locked"),
		ErrVersionConflict,
	} {
		if !isStoreHistoryTransientConcurrencyError(err) {
			t.Fatalf("%v was not retryable", err)
		}
	}
	for _, err := range []error{context.Canceled, context.DeadlineExceeded, errors.New("constraint failed"), storeHistorySQLStateError{state: "23505"}} {
		if isStoreHistoryTransientConcurrencyError(err) {
			t.Fatalf("%v was unexpectedly retryable", err)
		}
	}
}

type storeHistorySQLStateError struct{ state string }

func (err storeHistorySQLStateError) Error() string    { return err.state }
func (err storeHistorySQLStateError) SQLState() string { return err.state }
