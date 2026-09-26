package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	memberstore "task-processor/internal/integration/persistence/organization/membership"
)

func install(ctx context.Context, dsn string) error {
	if ctx == nil || strings.TrimSpace(dsn) == "" || !strings.HasPrefix(dsn, "postgresql://") && !strings.HasPrefix(dsn, "postgres://") {
		return fmt.Errorf("invalid membership database DSN")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return fmt.Errorf("open membership database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("open membership database pool: %w", err)
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping membership database: %w", err)
	}
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin membership schema transaction: %w", err)
	}
	if err := memberstore.InstallSchemaTx(ctx, tx); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("install membership schema: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit membership schema: %w", err)
	}
	return nil
}

func run(args []string, output io.Writer, installFn func(context.Context, string) error) int {
	fail := func() int { _, _ = fmt.Fprintln(output, "membership schema initialization failed"); return 1 }
	flags := flag.NewFlagSet("organization-membership-schema-init", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	path := flags.String("dsn-file", "", "explicit connection file")
	timeout := flags.Duration("timeout", 30*time.Second, "bounded installation timeout")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *path == "" || *timeout <= 0 || *timeout > time.Minute || installFn == nil {
		return fail()
	}
	f, err := os.Open(*path)
	if err != nil {
		return fail()
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 65536 {
		return fail()
	}
	data, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil || len(data) > 65536 || strings.TrimSpace(string(data)) == "" {
		return fail()
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if installFn(ctx, strings.TrimSpace(string(data))) != nil {
		return fail()
	}
	return 0
}

func main() { os.Exit(run(os.Args[1:], os.Stderr, install)) }
