package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"task-processor/internal/app/httpapi"
	"task-processor/internal/app/runtime/currentapplication"
	platformdatabase "task-processor/internal/platform/database"
)

func main() {
	if err := execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func execute() error {
	manifestPath := flag.String("config", "", "absolute path to the private current-application JSON manifest")
	shutdownFile := flag.String("shutdown-file", "", "optional absolute path to a private graceful-shutdown trigger")
	flag.Parse()
	if *manifestPath == "" {
		return fmt.Errorf("-config is required")
	}
	cfg, err := currentapplication.LoadConfig(*manifestPath)
	if err != nil {
		return err
	}
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *shutdownFile != "" {
		var cancel context.CancelFunc
		ctx, cancel, err = currentapplication.ContextWithShutdownFile(ctx, *shutdownFile)
		if err != nil {
			return err
		}
		defer cancel()
	}
	return currentapplication.Run(ctx, cfg, logger, currentapplication.Dependencies{
		OpenSourceAccount: func(cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritable(databaseConfig(cfg))
		},
		OpenCommercial: func(cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingReadOnly(databaseConfig(cfg))
		},
		NewApplication: httpapi.NewCurrentApplication,
		CloseDatabase:  platformdatabase.Close,
	})
}

func databaseConfig(cfg currentapplication.DatabaseConfig) *platformdatabase.Config {
	return &platformdatabase.Config{
		Host: cfg.Host, Port: cfg.Port, User: cfg.User, Password: cfg.Password, Database: cfg.Database,
		MaxConnections: cfg.MaxConnections, MaxIdleConnections: cfg.MaxConnections, ConnectionMaxLifetime: time.Hour,
	}
}
