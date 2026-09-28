package storecenter

import (
	"context"
	"database/sql"
	"errors"

	"github.com/pressly/goose/v3"
	"gorm.io/gorm"

	platformmigration "task-processor/internal/platform/database/migration"
	"task-processor/internal/storecenter"
)

func Migrate(ctx context.Context, db *gorm.DB) error {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return errors.New("store schema requires PostgreSQL")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	runner, err := platformmigration.NewWithVersionTable(goose.DialectPostgres, sqlDB, "public.goose_store_center_version",
		goose.NewGoMigration(2026092801, &goose.GoFunc{RunTx: func(ctx context.Context, tx *sql.Tx) error { return storecenter.InstallCurrentSchemaTx(ctx, tx) }}, nil))
	if err != nil {
		return err
	}
	if _, err = runner.Up(ctx); err != nil {
		return err
	}
	return storecenter.VerifyCurrentSchema(ctx, db)
}
