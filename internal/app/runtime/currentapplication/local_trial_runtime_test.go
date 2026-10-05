package currentapplication

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	coreconfig "task-processor/internal/core/config"
)

func issue36RuntimeConfig() *Config {
	cfg := storeRuntimeConfig()
	cfg.Identity.IssuerURL = "https://localhost:18444"
	cfg.Identity.AuthorizationAPIURL = cfg.Identity.IssuerURL
	cfg.StoreCenter.Database.Port = 5433
	cfg.StoreCenter.Database.Database = "store_center"
	cfg.LocalTrial = &LocalTrialConfig{Enabled: true, Database: DatabaseConfig{
		Host: "127.0.0.1", Port: 5433, User: "issue36_trial_runtime",
		Password: "synthetic-trial-secret", Database: "store_center", MaxConnections: 4,
	}}
	return cfg
}

func TestIssue36RuntimeOpensIndependentPoolAndClosesOnCompositionFailure(t *testing.T) {
	cfg := issue36RuntimeConfig()
	source, commercial, store, trial := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
	stop := errors.New("composition stopped")
	var closed []*gorm.DB
	err := Run(context.Background(), cfg, logrus.New(), Dependencies{
		IdentityPreflight:   func(context.Context, IdentityConfig) error { return nil },
		OpenSourceAccount:   func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil },
		OpenCommercialOwner: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return commercial, nil },
		OpenStoreCenter: func(_ context.Context, db DatabaseConfig) (*gorm.DB, error) {
			require.Equal(t, cfg.StoreCenter.Database, db)
			return store, nil
		},
		OpenLocalTrial: func(_ context.Context, db DatabaseConfig) (*gorm.DB, error) {
			require.Equal(t, cfg.LocalTrial.Database, db)
			return trial, nil
		},
		CloseDatabase: func(db *gorm.DB) error { closed = append(closed, db); return nil },
		NewApplicationWithFeatures: func(_ context.Context, got *gorm.DB, features ApplicationFeatures, _ *coreconfig.Config, _ *logrus.Logger) (*http.Server, error) {
			require.Same(t, source, got)
			require.Same(t, store, features.StoreCenterDB)
			require.Same(t, trial, features.LocalTrialDB)
			return nil, stop
		},
	})
	require.ErrorIs(t, err, stop)
	require.Equal(t, []*gorm.DB{trial, store, commercial, source}, closed)
}

func TestIssue36RuntimeFailsClosedWithoutDedicatedOpenerOrPool(t *testing.T) {
	for _, missing := range []string{"opener", "pool"} {
		t.Run(missing, func(t *testing.T) {
			cfg := issue36RuntimeConfig()
			source, commercial, store := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
			openedSource := false
			deps := Dependencies{
				IdentityPreflight: func(context.Context, IdentityConfig) error { return nil },
				OpenSourceAccount: func(context.Context, DatabaseConfig) (*gorm.DB, error) {
					openedSource = true
					return source, nil
				},
				OpenCommercialOwner: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return commercial, nil },
				OpenStoreCenter:     func(context.Context, DatabaseConfig) (*gorm.DB, error) { return store, nil },
				CloseDatabase:       func(*gorm.DB) error { return nil },
				NewApplicationWithFeatures: func(context.Context, *gorm.DB, ApplicationFeatures, *coreconfig.Config, *logrus.Logger) (*http.Server, error) {
					t.Fatal("partial trial must not serve")
					return nil, nil
				},
			}
			if missing == "pool" {
				deps.OpenLocalTrial = func(context.Context, DatabaseConfig) (*gorm.DB, error) { return store, nil }
			}
			require.Error(t, Run(context.Background(), cfg, logrus.New(), deps))
			if missing == "opener" {
				require.False(t, openedSource)
			}
		})
	}
}
