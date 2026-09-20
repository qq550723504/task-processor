package accountprofile

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
	store "task-processor/internal/integration/persistence/accountprofile"
	platformmigration "task-processor/internal/platform/database/migration"
)

const versionTableRelation = "public.goose_account_profile_version"
const baselineVersion = int64(2026092001)

func Migrations() []*goose.Migration {
	return []*goose.Migration{goose.NewGoMigration(baselineVersion, &goose.GoFunc{RunTx: func(ctx context.Context, tx *sql.Tx) error {
		return store.InstallSchemaTx(ctx, tx)
	}}, nil)}
}

func Migrate(ctx context.Context, db *gorm.DB) error {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return fmt.Errorf("account profile schema requires postgres")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get account profile database: %w", err)
	}
	runner, err := platformmigration.NewWithVersionTable(goose.DialectPostgres, sqlDB, versionTableRelation, Migrations()...)
	if err != nil {
		return fmt.Errorf("create account profile migration runner: %w", err)
	}
	if _, err := runner.Up(ctx); err != nil {
		return fmt.Errorf("initialize account profile schema: %w", err)
	}
	return store.VerifySchema(ctx, db)
}
