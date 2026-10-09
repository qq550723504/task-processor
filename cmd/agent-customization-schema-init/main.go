// agent-customization-schema-init consumes the admitted explicit schema installer.
// Serving never installs schema or creates sample requests.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	_ "github.com/jackc/pgx/v5/stdlib"
	"os"
	"regexp"
	store "task-processor/internal/integration/persistence/agentcustomization"
	"time"
)

func main() {
	role := flag.String("runtime-role", "", "existing restricted PostgreSQL serving role")
	flag.Parse()
	if flag.NArg() != 0 || !regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`).MatchString(*role) {
		fmt.Fprintln(os.Stderr, "requires --runtime-role with an existing role")
		os.Exit(2)
	}
	dsn := os.Getenv("AGENT_CUSTOMIZATION_SCHEMA_DSN")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "AGENT_CUSTOMIZATION_SCHEMA_DSN required")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "schema database unavailable")
		os.Exit(1)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = store.InstallSchema(ctx, db); err == nil {
		err = store.GrantRuntime(ctx, db, *role)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "agent customization schema initialization failed")
		os.Exit(1)
	}
	fmt.Println("agent customization schema ready; no business requests created")
}
