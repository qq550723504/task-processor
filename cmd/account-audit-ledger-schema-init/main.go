// Installs the existing invocation ledger in one fresh Agent owner database.
// Serving reads it through a different, SELECT-only role.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	store "task-processor/internal/aicapability/store"
)

func main() {
	path := flag.String("dsn-file", "", "private owner DSN file")
	namespace := flag.String("namespace", "", "existing invocation owner: image or product")
	flag.Parse()
	if flag.NArg() != 0 || *path == "" || (*namespace != "image" && *namespace != "product") {
		fmt.Fprintln(os.Stderr, "bounded invocation owner and private DSN file required")
		os.Exit(2)
	}
	data, err := os.ReadFile(*path)
	if err != nil || len(data) > 4096 || !strings.HasPrefix(strings.TrimSpace(string(data)), "postgresql://") {
		fmt.Fprintln(os.Stderr, "owner DSN unavailable")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := gorm.Open(postgres.Open(strings.TrimSpace(string(data))), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fmt.Fprintln(os.Stderr, "ledger owner unavailable")
		os.Exit(1)
	}
	raw, err := db.DB()
	if err != nil {
		os.Exit(1)
	}
	defer raw.Close()
	raw.SetMaxOpenConns(1)
	if err := raw.PingContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "ledger owner unavailable")
		os.Exit(1)
	}
	var databaseName, roleName string
	if err := db.WithContext(ctx).Raw("SELECT current_database(), current_user").Row().Scan(&databaseName, &roleName); err != nil || databaseName != *namespace+"_agent" || roleName != *namespace+"_agent_owner" {
		fmt.Fprintln(os.Stderr, "wrong invocation ledger owner")
		os.Exit(1)
	}
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return store.AutoMigrateInvocationLedger(tx) }); err != nil {
		fmt.Fprintln(os.Stderr, "ledger schema initialization failed")
		os.Exit(1)
	}
}
