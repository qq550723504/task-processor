package sourceaccountregistry

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
	"gorm.io/gorm"

	sourceaccountstore "task-processor/internal/integration/persistence/sourceaccountregistry"
	platformmigration "task-processor/internal/platform/database/migration"
)

const (
	VersionTableName = "goose_source_account_registry_version"
	baselineVersion  = int64(2026090901)
)

func Migrations() []*goose.Migration {
	return []*goose.Migration{goose.NewGoMigration(baselineVersion, &goose.GoFunc{RunTx: func(ctx context.Context, tx *sql.Tx) error {
		return sourceaccountstore.InstallSchemaTx(ctx, tx)
	}}, nil)}
}

func Migrate(ctx context.Context, db *gorm.DB) error {
	if db == nil || db.Dialector == nil {
		return fmt.Errorf("source account registry database is nil")
	}
	if db.Dialector.Name() != "postgres" {
		return fmt.Errorf("source account registry schema requires postgres, got %q", db.Dialector.Name())
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get source account registry database: %w", err)
	}
	runner, err := platformmigration.NewWithVersionTable(goose.DialectPostgres, sqlDB, VersionTableName, Migrations()...)
	if err != nil {
		return fmt.Errorf("create source account registry migration runner: %w", err)
	}
	if _, err := runner.Up(ctx); err != nil {
		return fmt.Errorf("initialize source account registry schema: %w", err)
	}
	if err := sourceaccountstore.VerifySchema(ctx, db); err != nil {
		return fmt.Errorf("verify source account registry schema: %w", err)
	}
	return nil
}
