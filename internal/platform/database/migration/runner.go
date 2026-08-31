package migration

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

type Runner struct {
	provider *goose.Provider
}

func New(dialect goose.Dialect, db *sql.DB, migrations ...*goose.Migration) (*Runner, error) {
	provider, err := goose.NewProvider(dialect, db, nil, goose.WithGoMigrations(migrations...))
	if err != nil {
		return nil, fmt.Errorf("create migration provider: %w", err)
	}
	return &Runner{provider: provider}, nil
}

func (r *Runner) Up(ctx context.Context) ([]*goose.MigrationResult, error) {
	return r.provider.Up(ctx)
}

func (r *Runner) Status(ctx context.Context) ([]*goose.MigrationStatus, error) {
	return r.provider.Status(ctx)
}
