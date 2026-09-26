package sourceaccountregistry

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
	"gorm.io/gorm"

	accountprofileschema "task-processor/internal/app/schema/accountprofile"
	sourceaccountstore "task-processor/internal/integration/persistence/sourceaccountregistry"
	verificationstore "task-processor/internal/integration/persistence/subjectverification"
	platformmigration "task-processor/internal/platform/database/migration"
)

const (
	VersionTableName            = "goose_source_account_registry_version"
	versionTableRelation        = "public." + VersionTableName
	baselineVersion             = int64(2026090901)
	verificationVersion         = int64(2026092601)
	personalVerificationVersion = int64(2026092701)
)

func Migrations() []*goose.Migration {
	return []*goose.Migration{
		goose.NewGoMigration(baselineVersion, &goose.GoFunc{RunTx: func(ctx context.Context, tx *sql.Tx) error {
			return sourceaccountstore.InstallSchemaTx(ctx, tx)
		}}, nil),
		goose.NewGoMigration(verificationVersion, &goose.GoFunc{RunTx: func(ctx context.Context, tx *sql.Tx) error {
			return verificationstore.InstallSchemaTx(ctx, tx)
		}}, nil),
		goose.NewGoMigration(personalVerificationVersion, &goose.GoFunc{RunTx: func(ctx context.Context, tx *sql.Tx) error {
			return verificationstore.InstallPersonalSchemaTx(ctx, tx)
		}}, nil),
	}
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
	runner, err := platformmigration.NewWithVersionTable(goose.DialectPostgres, sqlDB, versionTableRelation, Migrations()...)
	if err != nil {
		return fmt.Errorf("create source account registry migration runner: %w", err)
	}
	if _, err := runner.Up(ctx); err != nil {
		return fmt.Errorf("initialize source account registry schema: %w", err)
	}
	if err := sourceaccountstore.VerifySchema(ctx, db); err != nil {
		return fmt.Errorf("verify source account registry schema: %w", err)
	}
	if err := accountprofileschema.Migrate(ctx, db); err != nil {
		return fmt.Errorf("initialize account profile schema: %w", err)
	}
	_, err = verificationstore.NewRepository(ctx, db)
	if err != nil {
		return err
	}
	_, err = verificationstore.NewPersonalRepository(ctx, db)
	return err
}
