// agent-configuration-schema-init installs the admitted fresh-install schema in
// the existing ProductAgentDB. Serving never performs DDL or auto-enables agents.
package main

import (
	"context"
	"flag"
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"regexp"
	store "task-processor/internal/integration/persistence/agentconfig"
	"time"
)

func main() {
	role := flag.String("runtime-role", "", "existing runtime PostgreSQL role to grant bounded privileges")
	flag.Parse()
	if flag.NArg() != 0 || !regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`).MatchString(*role) {
		fmt.Fprintln(os.Stderr, "requires --runtime-role with an existing role")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dsn := os.Getenv("AGENT_CONFIGURATION_SCHEMA_DSN")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "AGENT_CONFIGURATION_SCHEMA_DSN required")
		os.Exit(2)
	}
	db, e := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		fmt.Fprintln(os.Stderr, "schema database unavailable")
		os.Exit(1)
	}
	raw, e := db.DB()
	if e != nil {
		os.Exit(1)
	}
	defer raw.Close()
	e = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := store.InstallSchema(tx); e != nil {
			return e
		}
		return store.GrantRuntime(tx, *role)
	})
	if e != nil {
		fmt.Fprintln(os.Stderr, "agent configuration schema initialization failed")
		os.Exit(1)
	}
	fmt.Println("agent configuration schema ready; no enterprise activation created")
}
