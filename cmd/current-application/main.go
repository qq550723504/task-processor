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

	"net/http"
	"task-processor/internal/app/httpapi"
	"task-processor/internal/app/runtime/currentapplication"
	coreconfig "task-processor/internal/core/config"
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
		IdentityPreflight: currentapplication.VerifyIdentityProvider,
		OpenSourceAccount: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritableContext(ctx, databaseConfig(cfg))
		},
		OpenCommercial: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingReadOnlyContext(ctx, databaseConfig(cfg))
		},
		NewApplication: httpapi.NewCurrentApplication,
		OpenMembership: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritableContext(ctx, databaseConfig(cfg))
		},
		NewApplicationWithMembership: func(ctx context.Context, source, commercial, membershipDB *gorm.DB, cfg *coreconfig.Config, membership *currentapplication.MembershipConfig, logger *logrus.Logger) (*http.Server, error) {
			return httpapi.NewCurrentApplicationWithMembership(ctx, source, commercial, cfg, logger, httpapi.MembershipDependencies{ReceiptDB: membershipDB, ProviderOrigin: membership.ProviderOrigin, ReadToken: membership.ReadToken, WriteToken: membership.WriteToken})
		},
		CloseDatabase: platformdatabase.Close,
	})
}

func databaseConfig(cfg currentapplication.DatabaseConfig) *platformdatabase.Config {
	return &platformdatabase.Config{
		Host: cfg.Host, Port: cfg.Port, User: cfg.User, Password: cfg.Password, Database: cfg.Database,
		MaxConnections: cfg.MaxConnections, MaxIdleConnections: cfg.MaxConnections, ConnectionMaxLifetime: time.Hour,
	}
}
