package productlisting

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
	"gorm.io/gorm"

	platformmigration "task-processor/internal/platform/database/migration"
)

const baselineMigrationVersion int64 = 2026083001

// Migrations returns the immutable product-listing schema history. The baseline
// deliberately has no down migration because dropping existing production
// tables is unsafe. Future schema changes must use a new Goose version instead
// of changing this published migration.
func Migrations(db *gorm.DB) []*goose.Migration {
	baseline := goose.NewGoMigration(baselineMigrationVersion, &goose.GoFunc{
		RunDB: func(ctx context.Context, migrationDB *sql.DB) error {
			if db == nil {
				return fmt.Errorf("database is nil")
			}
			runtimeDB, err := db.DB()
			if err != nil {
				return fmt.Errorf("get product listing database: %w", err)
			}
			if runtimeDB != migrationDB {
				return fmt.Errorf("migration sql.DB does not match product listing gorm database")
			}
			return AutoMigrateRuntime(db.WithContext(ctx))
		},
	}, nil)
	return []*goose.Migration{baseline}
}

func Migrate(ctx context.Context, db *gorm.DB) error {
	dialect, err := resolveDialect(db)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get product listing database: %w", err)
	}
	runner, err := platformmigration.New(dialect, sqlDB, Migrations(db)...)
	if err != nil {
		return fmt.Errorf("create product listing migration runner: %w", err)
	}
	if _, err := runner.Up(ctx); err != nil {
		return fmt.Errorf("migrate product listing schema: %w", err)
	}
	return nil
}

func resolveDialect(db *gorm.DB) (goose.Dialect, error) {
	if db == nil {
		return "", fmt.Errorf("database is nil")
	}
	if db.Dialector == nil {
		return "", fmt.Errorf("database dialect is nil")
	}
	switch name := db.Dialector.Name(); name {
	case "sqlite":
		return goose.DialectSQLite3, nil
	case "postgres":
		return goose.DialectPostgres, nil
	default:
		return "", fmt.Errorf("unsupported database dialect %q", name)
	}
}
