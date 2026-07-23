package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/core/config"
	corelogger "task-processor/internal/core/logger"
	"task-processor/internal/infra/database"
	listingkitstore "task-processor/internal/listingkit/store"
)

const defaultBatchSize = 200

type options struct {
	configPath string
	batchSize  int
}

type dependencies struct {
	loadConfig func(string) (*config.Config, error)
	openDB     func(*config.DatabaseConfig) (*gorm.DB, error)
	migrate    func(*gorm.DB) error
	backfill   func(context.Context, *gorm.DB, int) (int64, error)
	closeDB    func(*gorm.DB) error
	now        func() time.Time
}

func main() {
	os.Exit(runMain(
		context.Background(),
		os.Args[1:],
		os.Stdout,
		os.Stderr,
		defaultDependencies(),
	))
}

func runMain(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	deps dependencies,
) int {
	if err := run(ctx, args, stdout, deps); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func parseArgs(args []string) (options, error) {
	opts := options{}
	fs := flag.NewFlagSet("listingkit-shein-pod-image-index-backfill", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.configPath, "config", "", "config path (required)")
	fs.IntVar(&opts.batchSize, "batch-size", defaultBatchSize, "tasks per transaction")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	opts.configPath = strings.TrimSpace(opts.configPath)
	if opts.configPath == "" {
		return options{}, fmt.Errorf("-config is required")
	}
	return opts, nil
}

func defaultDependencies() dependencies {
	return dependencies{
		loadConfig: loadConfigWithoutGlobalLogs,
		openDB:     database.NewDatabaseFromConfig,
		migrate:    listingkitstore.AutoMigrateSheinPODImageLookupIndex,
		backfill:   listingkitstore.BackfillSheinPODImageLookupIndexes,
		closeDB:    database.CloseDatabase,
		now:        time.Now,
	}
}

func loadConfigWithoutGlobalLogs(configPath string) (*config.Config, error) {
	globalLogger := corelogger.GetGlobalLogManager().GetRawLogger()
	originalOutput := globalLogger.Out
	globalLogger.SetOutput(io.Discard)
	defer globalLogger.SetOutput(originalOutput)
	return config.LoadConfigFromFile(configPath)
}

func run(ctx context.Context, args []string, output io.Writer, deps dependencies) (returnErr error) {
	opts, err := parseArgs(args)
	if err != nil {
		return err
	}
	cfg, err := deps.loadConfig(opts.configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg == nil || cfg.Database == nil {
		return fmt.Errorf("database config is required")
	}
	db, err := deps.openDB(cfg.Database)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	if db == nil {
		return fmt.Errorf("database is required")
	}
	defer func() {
		if err := deps.closeDB(db); returnErr == nil && err != nil {
			returnErr = fmt.Errorf("close database: %w", err)
		}
	}()

	if err := deps.migrate(db); err != nil {
		return fmt.Errorf("migrate POD image lookup index schema: %w", err)
	}
	startedAt := deps.now()
	processed, err := deps.backfill(ctx, db, opts.batchSize)
	if err != nil {
		return fmt.Errorf("backfill POD image lookup indexes: %w", err)
	}
	duration := deps.now().Sub(startedAt)
	_, err = fmt.Fprintf(output, "processed=%d duration=%s\n", processed, duration)
	return err
}
