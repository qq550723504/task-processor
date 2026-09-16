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

func acquisitionRuntimeConfig() *Config {
	return &Config{SchemaVersion: 1, Listen: ListenConfig{Host: "127.0.0.1", Port: 18081}, Identity: IdentityConfig{IssuerURL: "http://127.0.0.1:18080", AuthorizationAPIURL: "http://127.0.0.1:18080", ClientID: "fixture-client", ClientSecret: "fixture-secret", ProjectID: "fixture-project"},
		SourceAccountDatabase:      DatabaseConfig{Host: "127.0.0.1", Port: 5432, User: "source_account_runtime", Password: "fixture-password", Database: "source_account", MaxConnections: 2},
		CommercialDatabase:         DatabaseConfig{Host: "127.0.0.1", Port: 5432, User: "commercial_reader", Password: "fixture-password", Database: "commercial", MaxConnections: 2},
		ProductAcquisitionDatabase: &DatabaseConfig{Host: "127.0.0.1", Port: 5432, User: "source_acquisition_runtime", Password: "fixture-password", Database: "product", MaxConnections: 8}}
}

func TestAcquisitionRuntimeConfigRequiresDedicatedBoundedRole(t *testing.T) {
	cfg := acquisitionRuntimeConfig()
	require.NoError(t, cfg.validate())
	for _, mutate := range []func(*Config){
		func(c *Config) { c.ProductAcquisitionDatabase.MaxConnections = 9 },
		func(c *Config) { c.ProductAcquisitionDatabase.User = "source_account_runtime" },
		func(c *Config) { c.ProductAcquisitionDatabase.Database = c.SourceAccountDatabase.Database },
		func(c *Config) { c.ProductAcquisitionDatabase.Database = c.CommercialDatabase.Database },
	} {
		cfg := acquisitionRuntimeConfig()
		mutate(cfg)
		require.Error(t, cfg.validate())
	}
	cfg.ProductAcquisitionDatabase = nil
	require.NoError(t, cfg.validate(), "existing RUN-1 does not implicitly enable acquisition")
}

func TestAcquisitionRuntimeOwnsThirdPoolWithoutLegacyFallback(t *testing.T) {
	cfg := acquisitionRuntimeConfig()
	source, commercial, product := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
	var closed []*gorm.DB
	newCalled, oldCalled := false, false
	dependencies := Dependencies{
		IdentityPreflight: func(context.Context, IdentityConfig) error { return nil },
		OpenSourceAccount: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil },
		OpenCommercial:    func(context.Context, DatabaseConfig) (*gorm.DB, error) { return commercial, nil },
		OpenProductAcquisition: func(_ context.Context, db DatabaseConfig) (*gorm.DB, error) {
			require.Equal(t, *cfg.ProductAcquisitionDatabase, db)
			return product, nil
		},
		NewApplication: func(context.Context, *gorm.DB, *gorm.DB, *coreconfig.Config, *logrus.Logger) (*http.Server, error) {
			oldCalled = true
			return nil, errors.New("unexpected base constructor")
		},
		NewApplicationWithAcquisition: func(_ context.Context, a, b, c *gorm.DB, _ *coreconfig.Config, _ *logrus.Logger) (*http.Server, error) {
			require.Same(t, source, a)
			require.Same(t, commercial, b)
			require.Same(t, product, c)
			newCalled = true
			return nil, errors.New("stop acquisition before listener")
		},
		CloseDatabase: func(db *gorm.DB) error { closed = append(closed, db); return nil },
	}
	err := Run(context.Background(), cfg, logrus.New(), dependencies)
	require.ErrorContains(t, err, "stop acquisition before listener")
	require.True(t, newCalled)
	require.False(t, oldCalled)
	require.Equal(t, []*gorm.DB{product, commercial, source}, closed)
	dependencies.OpenProductAcquisition = nil
	err = Run(context.Background(), cfg, logrus.New(), dependencies)
	require.Error(t, err)
	require.False(t, oldCalled, "configured acquisition must fail closed, not drop its module")
}
