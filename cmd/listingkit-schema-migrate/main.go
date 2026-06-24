package main

import (
	"flag"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	"task-processor/internal/listingkit/httpapi"
	listingkitstore "task-processor/internal/listingkit/store"
	"task-processor/internal/pkg/appenv"

	"gorm.io/gorm"
)

var (
	configPath = flag.String("config", "config/config-dev.yaml", "config file path")
	logLevel   = flag.String("log-level", "info", "log level")
	scope      = flag.String("scope", "all", "migration scope: all or shein-sync")
)

func main() {
	flag.Parse()

	logger := appenv.SetupLoggerWithLevel(*logLevel)
	cfg, err := config.LoadConfigFromFile(*configPath)
	if err != nil {
		logger.Fatalf("load config failed: %v", err)
	}
	db, err := database.NewDatabaseFromConfig(cfg.Database)
	if err != nil {
		logger.Fatalf("connect database failed: %v", err)
	}
	if db == nil {
		logger.Fatal("database config is required")
	}
	sqlDB, err := db.DB()
	if err != nil {
		logger.Fatalf("get database handle failed: %v", err)
	}
	defer sqlDB.Close()

	if err := runMigration(db); err != nil {
		logger.Fatalf("listingkit schema migrate failed: %v", err)
	}
	logger.WithField("scope", *scope).Info("listingkit schema migrate completed")
}

func runMigration(db *gorm.DB) error {
	switch *scope {
	case "all":
		return httpapi.AutoMigrateListingKitRuntimeSchema(db)
	case "shein-sync":
		return listingkitstore.AutoMigrateSheinSyncRepository(db)
	default:
		return flag.ErrHelp
	}
}
