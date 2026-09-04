package orgresourceadapter

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/ledger/orgresource"
)

type TransactionConfig struct {
	LockTimeout        time.Duration
	StatementTimeout   time.Duration
	TransactionTimeout time.Duration
	TotalRetryBudget   time.Duration
	BaseRetryDelay     time.Duration
	MaxAttempts        int
}

func (config TransactionConfig) withDefaults() TransactionConfig {
	if config.LockTimeout <= 0 {
		config.LockTimeout = 500 * time.Millisecond
	}
	if config.StatementTimeout <= 0 {
		config.StatementTimeout = 2 * time.Second
	}
	if config.TransactionTimeout <= 0 {
		config.TransactionTimeout = 3 * time.Second
	}
	if config.TotalRetryBudget <= 0 {
		config.TotalRetryBudget = 5 * time.Second
	}
	if config.BaseRetryDelay <= 0 {
		config.BaseRetryDelay = 20 * time.Millisecond
	}
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 4
	}
	return config
}

type transactionRunner struct {
	db      *gorm.DB
	config  TransactionConfig
	dialect string
}

func newTransactionRunner(db *gorm.DB, config TransactionConfig) *transactionRunner {
	return &transactionRunner{db: db, config: config.withDefaults(), dialect: db.Dialector.Name()}
}

func (runner *transactionRunner) run(ctx context.Context, operation func(*gorm.DB) error) error {
	return runner.runWithRetry(ctx, func(transactionContext context.Context) error {
		return runner.db.WithContext(transactionContext).Transaction(func(tx *gorm.DB) error {
			if runner.dialect == "postgres" {
				if setErr := setPostgresLocalTimeouts(tx, runner.config); setErr != nil {
					return setErr
				}
			}
			return operation(tx)
		}, &sql.TxOptions{Isolation: sql.LevelSerializable})
	})
}

// runRead applies the same bounded transient-concurrency policy to durable
// replay and authoritative read-back queries. Those reads run outside the
// write transaction by design, but they can still observe SQLite busy or
// PostgreSQL concurrency cancellation while another attempt commits.
func (runner *transactionRunner) runRead(ctx context.Context, operation func(context.Context) error) error {
	return runner.runWithRetry(ctx, operation)
}

func (runner *transactionRunner) runWithRetry(ctx context.Context, operation func(context.Context) error) error {
	budgetContext, cancelBudget := context.WithTimeout(ctx, runner.config.TotalRetryBudget)
	defer cancelBudget()

	var lastErr error
	for attempt := 1; attempt <= runner.config.MaxAttempts; attempt++ {
		if err := budgetContext.Err(); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("%w: %v", orgresource.ErrConcurrencyRetry, lastErr)
		}
		attemptContext, cancelAttempt := context.WithTimeout(budgetContext, runner.config.TransactionTimeout)
		err := operation(attemptContext)
		attemptContextErr := attemptContext.Err()
		budgetContextErr := budgetContext.Err()
		cancelAttempt()
		if err == nil {
			return nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return ctx.Err()
		}
		runnerDeadline := errors.Is(err, context.DeadlineExceeded) &&
			(errors.Is(attemptContextErr, context.DeadlineExceeded) || errors.Is(budgetContextErr, context.DeadlineExceeded))
		if !runnerDeadline && !runner.retryable(err) {
			return err
		}
		if attempt == runner.config.MaxAttempts {
			break
		}
		delay := runner.retryDelay(attempt)
		timer := time.NewTimer(delay)
		select {
		case <-budgetContext.Done():
			if !timer.Stop() {
				<-timer.C
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("%w: %v", orgresource.ErrConcurrencyRetry, lastErr)
		case <-timer.C:
		}
	}
	return fmt.Errorf("%w: %v", orgresource.ErrConcurrencyRetry, lastErr)
}

func setPostgresLocalTimeouts(tx *gorm.DB, config TransactionConfig) error {
	settings := []struct {
		name  string
		value time.Duration
	}{
		{name: "lock_timeout", value: config.LockTimeout},
		{name: "statement_timeout", value: config.StatementTimeout},
	}
	for _, setting := range settings {
		value := fmt.Sprintf("%dms", max(setting.value.Milliseconds(), 1))
		if err := tx.Exec("SELECT set_config(?, ?, true)", setting.name, value).Error; err != nil {
			return fmt.Errorf("set PostgreSQL %s: %w", setting.name, err)
		}
	}
	return nil
}

func (runner *transactionRunner) retryable(err error) bool {
	state := sqlState(err)
	switch state {
	case "40001", "40P01", "55P03":
		return true
	case "57014":
		return true
	}
	return runner.dialect == "sqlite" && strings.Contains(strings.ToLower(err.Error()), "database is locked")
}

func (runner *transactionRunner) retryDelay(attempt int) time.Duration {
	multiplier := 1 << min(attempt-1, 6)
	base := runner.config.BaseRetryDelay * time.Duration(multiplier)
	jitterCeiling := max(base/2, time.Millisecond)
	return base + time.Duration(rand.Int63n(int64(jitterCeiling)))
}

type sqlStateError interface {
	SQLState() string
}

func sqlState(err error) string {
	for current := err; current != nil; current = errors.Unwrap(current) {
		if stateErr, ok := current.(sqlStateError); ok {
			return stateErr.SQLState()
		}
	}
	return ""
}
