package main

import (
	"database/sql"
	"flag"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	"task-processor/internal/pkg/appenv"

	"gorm.io/gorm"
)

var (
	appConfig = flag.String("app-config", "config/config-dev.yaml", "application config path")
	logLevel  = flag.String("log-level", "info", "log level")
)

var (
	appVersion = "1.0.0"
	buildTime  = "unknown"
)

type dbMeta struct {
	Version         string
	CurrentDB       string
	CurrentUser     string
	ServerTime      time.Time
	ServerAddr      sql.NullString
	ServerPort      sql.NullInt64
	ClientAddr      sql.NullString
	TransactionRead sql.NullString
}

func main() {
	flag.Parse()

	logger := appenv.SetupLoggerWithLevel(*logLevel)
	appenv.PrintVersionInfo(logger, appenv.VersionInfo{
		Version:   appVersion,
		BuildTime: buildTime,
	})

	configPath := normalizeConfigPath(*appConfig)
	logger.Infof("loading config: %s", configPath)

	cfg := config.LoadConfigWithFallback(configPath, logger)
	if cfg.Database == nil {
		logger.Fatal("database config is missing")
	}

	logger.Infof("testing database connection: host=%s port=%d database=%s user=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.Database, cfg.Database.User)

	db, err := database.NewDatabaseFromConfig(cfg.Database)
	if err != nil {
		logger.Fatalf("database connection failed: %v", err)
	}
	defer func() {
		if closeErr := database.CloseDatabase(db); closeErr != nil {
			logger.Warnf("close database failed: %v", closeErr)
		}
	}()

	sqlDB, err := db.DB()
	if err != nil {
		logger.Fatalf("get sql.DB failed: %v", err)
	}

	stats := sqlDB.Stats()
	logger.Infof("database connection succeeded")
	logger.Infof("pool settings: max_open=%d open=%d in_use=%d idle=%d",
		stats.MaxOpenConnections, stats.OpenConnections, stats.InUse, stats.Idle)

	meta, err := queryMeta(db)
	if err != nil {
		logger.Fatalf("database metadata query failed: %v", err)
	}

	logger.Infof("database version: %s", meta.Version)
	logger.Infof("current database: %s", meta.CurrentDB)
	logger.Infof("current user: %s", meta.CurrentUser)
	logger.Infof("server time: %s", meta.ServerTime.Format(time.RFC3339))

	if meta.ServerAddr.Valid || meta.ServerPort.Valid {
		logger.Infof("server address: %s:%d", nullString(meta.ServerAddr), meta.ServerPort.Int64)
	}
	if meta.ClientAddr.Valid {
		logger.Infof("client address: %s", meta.ClientAddr.String)
	}
	if meta.TransactionRead.Valid {
		logger.Infof("transaction_read_only: %s", meta.TransactionRead.String)
	}
}

func normalizeConfigPath(path string) string {
	if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
		return path
	}
	return fmt.Sprintf("%s.yaml", path)
}

func queryMeta(db *gorm.DB) (*dbMeta, error) {
	meta := &dbMeta{}

	err := db.Raw(`
		SELECT
			version() AS version,
			current_database() AS current_db,
			current_user AS current_user,
			now() AS server_time,
			inet_server_addr()::text AS server_addr,
			inet_server_port() AS server_port,
			inet_client_addr()::text AS client_addr,
			current_setting('transaction_read_only') AS transaction_read
	`).Scan(meta).Error
	if err != nil {
		return nil, err
	}

	return meta, nil
}

func nullString(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}
