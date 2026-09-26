package subjectverification

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
	store "task-processor/internal/integration/persistence/subjectverification"
	platformmigration "task-processor/internal/platform/database/migration"
)

func Migrate(ctx context.Context, db *gorm.DB) error {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return fmt.Errorf("verification schema requires postgres")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	runner, err := platformmigration.NewWithVersionTable(goose.DialectPostgres, sqlDB, "public.goose_subject_verification_version", goose.NewGoMigration(2026092601, &goose.GoFunc{RunTx: func(ctx context.Context, tx *sql.Tx) error { return store.InstallSchemaTx(ctx, tx) }}, nil))
	if err != nil {
		return err
	}
	if _, err = runner.Up(ctx); err != nil {
		return err
	}
	_, err = store.NewRepository(ctx, db)
	return err
}
