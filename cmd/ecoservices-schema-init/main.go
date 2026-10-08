// ecoservices-schema-init provisions only the dedicated greenfield E owner.
package main

import (
	"context"
	"errors"
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	store "task-processor/internal/integration/persistence/ecoservices"
	"time"
)

func initialize(ctx context.Context, db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return errors.New("dedicated PostgreSQL owner required")
	}
	var database string
	if err := db.WithContext(ctx).Raw("SELECT current_database()").Scan(&database).Error; err != nil || database != "ecoservices" {
		return errors.New("ecoservices database required")
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := store.Install(ctx, tx); err != nil {
			return err
		}
		return store.GrantRuntime(ctx, tx)
	})
}
func main() {
	if len(os.Args) != 1 || os.Getenv("ECOSERVICES_SCHEMA_DSN") == "" {
		fmt.Fprintln(os.Stderr, "requires ECOSERVICES_SCHEMA_DSN for the dedicated ecoservices database")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := gorm.Open(postgres.Open(os.Getenv("ECOSERVICES_SCHEMA_DSN")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fmt.Fprintln(os.Stderr, "schema owner database unavailable")
		os.Exit(1)
	}
	raw, err := db.DB()
	if err != nil {
		os.Exit(1)
	}
	defer raw.Close()
	if err = initialize(ctx, db); err != nil {
		fmt.Fprintln(os.Stderr, "ecoservices fresh schema initialization failed")
		os.Exit(1)
	}
	fmt.Println("ecoservices schema ready; no application, order or payment created")
}
