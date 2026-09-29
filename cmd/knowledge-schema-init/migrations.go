package main

import (
	"context"
	"database/sql"
	"errors"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
	persistence "task-processor/internal/integration/persistence/knowledge"
	migration "task-processor/internal/platform/database/migration"
)

func migrateKnowledge(ctx context.Context, db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return errors.New("knowledge schema requires PostgreSQL")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	runner, err := migration.NewWithVersionTable(goose.DialectPostgres, sqlDB, "public.goose_knowledge_version",
		goose.NewGoMigration(2026092801, &goose.GoFunc{RunTx: func(ctx context.Context, tx *sql.Tx) error { return persistence.InstallSchemaTx(ctx, tx) }}, nil))
	if err != nil {
		return err
	}
	if _, err = runner.Up(ctx); err != nil {
		return err
	}
	_, err = persistence.NewRepository(ctx, db)
	return err
}
