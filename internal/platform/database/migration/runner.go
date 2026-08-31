package migration

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

type Runner struct {
	provider *goose.Provider
	upMu     sync.Locker
}

// sqliteUpMu serializes the complete Goose Up operation across independently
// constructed SQLite providers in this process. SQLite support is intended for
// development and tests only: this mutex cannot coordinate separate processes.
var sqliteUpMu sync.Mutex

func New(dialect goose.Dialect, db *sql.DB, migrations ...*goose.Migration) (*Runner, error) {
	providerOptions := []goose.ProviderOption{goose.WithGoMigrations(migrations...)}
	var upMu sync.Locker
	if dialect == goose.DialectPostgres {
		sessionLocker, err := lock.NewPostgresSessionLocker()
		if err != nil {
			return nil, fmt.Errorf("create postgres migration session locker: %w", err)
		}
		providerOptions = append(providerOptions, goose.WithSessionLocker(sessionLocker))
	}
	if dialect == goose.DialectSQLite3 {
		upMu = &sqliteUpMu
	}

	provider, err := goose.NewProvider(dialect, db, nil, providerOptions...)
	if err != nil {
		return nil, fmt.Errorf("create migration provider: %w", err)
	}
	return &Runner{provider: provider, upMu: upMu}, nil
}

func (r *Runner) Up(ctx context.Context) ([]*goose.MigrationResult, error) {
	if r.upMu != nil {
		r.upMu.Lock()
		defer r.upMu.Unlock()
	}
	return r.provider.Up(ctx)
}

func (r *Runner) Status(ctx context.Context) ([]*goose.MigrationStatus, error) {
	return r.provider.Status(ctx)
}
