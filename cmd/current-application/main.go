package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

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
		OpenProductAcquisition: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritableContext(ctx, databaseConfig(cfg))
		},
		OpenReferrals: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritableContext(ctx, databaseConfig(cfg))
		},
		OpenMembership: func(ctx context.Context, cfg currentapplication.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenExistingWritableContext(ctx, databaseConfig(cfg))
		},
		NewApplicationWithFeatures: func(ctx context.Context, source, commercial *gorm.DB, features currentapplication.ApplicationFeatures, cfg *coreconfig.Config, logger *logrus.Logger) (*http.Server, error) {
			options := make([]httpapi.CurrentApplicationOption, 0, 3)
			if features.ProductAcquisitionDB != nil {
				options = append(options, httpapi.WithProductAcquisition(features.ProductAcquisitionDB))
			}
			if features.ReferralDB != nil {
				options = append(options, httpapi.WithReferrals(features.ReferralDB))
			}
			if features.MembershipDB != nil {
				if features.Membership == nil {
					return nil, fmt.Errorf("membership configuration unavailable")
				}
				options = append(options, httpapi.WithMembership(httpapi.MembershipDependencies{ReceiptDB: features.MembershipDB, ProviderOrigin: features.Membership.ProviderOrigin, ReadToken: features.Membership.ReadToken, WriteToken: features.Membership.WriteToken}))
			}
			return httpapi.NewCurrentApplicationWithOptions(ctx, source, commercial, cfg, logger, options...)
		},
		NewApplicationWithAcquisition:             httpapi.NewCurrentApplicationWithAcquisition,
		NewApplicationWithAcquisitionAndReferrals: httpapi.NewCurrentApplicationWithAcquisitionAndReferrals,
		NewApplication: func(ctx context.Context, source, commercial *gorm.DB, cfg *coreconfig.Config, logger *logrus.Logger) (*http.Server, error) {
			return httpapi.NewCurrentApplication(ctx, source, commercial, cfg, logger)
		},
		NewReferralsApplication: func(ctx context.Context, source, commercial, referrals *gorm.DB, cfg *coreconfig.Config, logger *logrus.Logger) (*http.Server, error) {
			return httpapi.NewCurrentApplicationWithOptions(ctx, source, commercial, cfg, logger, httpapi.WithReferrals(referrals))
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
