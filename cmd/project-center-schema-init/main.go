// project-center-schema-init installs the fresh-install Workbench schema in the
// Project Center database, then grants the pre-created bounded serving role.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	store "task-processor/internal/integration/persistence/aiworkbench/projectcenter"
)

func main() {
	role := flag.String("runtime-role", "", "existing restricted PostgreSQL runtime role")
	flag.Parse()
	if flag.NArg() != 0 || *role != store.RuntimeRole || !regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`).MatchString(*role) {
		fmt.Fprintln(os.Stderr, "requires --runtime-role with an existing role")
		os.Exit(2)
	}
	dsn := os.Getenv("PROJECT_CENTER_SCHEMA_DSN")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "PROJECT_CENTER_SCHEMA_DSN required")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fmt.Fprintln(os.Stderr, "schema database unavailable")
		os.Exit(1)
	}
	raw, err := db.DB()
	if err != nil {
		os.Exit(1)
	}
	defer raw.Close()
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := store.InstallSchema(tx); err != nil {
			return err
		}
		return store.GrantRuntime(tx)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Project Center schema initialization failed")
		os.Exit(1)
	}
	fmt.Println("Project Center schema ready; no business data created")
}
