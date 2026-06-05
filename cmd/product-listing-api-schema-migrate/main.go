package main

import (
	"flag"
	"fmt"
	"os"

	apphttpapi "task-processor/internal/app/httpapi"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
)

func main() {
	var configPath = flag.String("config", "config/config-dev.yaml", "config file path")
	flag.Parse()

	cfg, err := config.LoadConfigFromFile(*configPath)
	if err != nil {
		exitf("load config: %v", err)
	}
	if cfg.Database == nil {
		exitf("database is not configured")
	}

	db, err := database.NewDatabaseFromConfig(cfg.Database)
	if err != nil {
		exitf("connect database: %v", err)
	}
	if db == nil {
		exitf("database is not configured")
	}

	sqlDB, err := db.DB()
	if err == nil {
		defer sqlDB.Close()
	}

	if err := apphttpapi.AutoMigrateProductListingAPIRuntimeSchema(db); err != nil {
		exitf("migrate product listing API schema: %v", err)
	}

	fmt.Printf("product-listing-api schema migration completed using %s\n", *configPath)
}

func exitf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
