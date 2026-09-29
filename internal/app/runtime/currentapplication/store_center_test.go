package currentapplication

import (
	"context"
	"errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"net/http"
	coreconfig "task-processor/internal/core/config"
	"testing"
)

func storeRuntimeConfig() *Config {
	c := runtimeTestConfig()
	owner := c.SourceAccountDatabase
	owner.User = "commercial_owner_runtime"
	c.CommercialOwnerDatabase = &owner
	store := c.SourceAccountDatabase
	store.User, store.Database = "store_center_runtime", "stores"
	c.StoreCenter = &StoreCenterConfig{Enabled: true, Database: store}
	return c
}
func TestStoreCenterRequiresResourceOwnerAndNarrowNativePool(t *testing.T) {
	require.NoError(t, storeRuntimeConfig().validate())
	for name, mutate := range map[string]func(*Config){
		"missing resource owner": func(c *Config) { c.CommercialOwnerDatabase = nil },
		"wide record role":       func(c *Config) { c.StoreCenter.Database.User = "commercial_owner_runtime" },
		"shared record database": func(c *Config) { c.StoreCenter.Database.Database = c.SourceAccountDatabase.Database },
		"unbounded record pool":  func(c *Config) { c.StoreCenter.Database.MaxConnections = 9 },
	} {
		t.Run(name, func(t *testing.T) { c := storeRuntimeConfig(); mutate(c); require.Error(t, c.validate()) })
	}
	c := runtimeTestConfig()
	c.StoreCenter = &StoreCenterConfig{Enabled: false}
	require.NoError(t, c.validate())
}
func TestStoreCenterRuntimeClosesCurrentPoolsOnPartialStartup(t *testing.T) {
	for failAt := 0; failAt <= 2; failAt++ {
		t.Run(string(rune('0'+failAt)), func(t *testing.T) {
			cfg := storeRuntimeConfig()
			cfg.StoreCenter.Enabled = failAt != 0
			source, resource, records := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
			var closed []*gorm.DB
			opened := 0
			stop := errors.New("isolated startup stop")
			deps := Dependencies{
				IdentityPreflight:   func(context.Context, IdentityConfig) error { return nil },
				OpenSourceAccount:   func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil },
				OpenCommercialOwner: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return resource, nil },
				OpenStoreCenter: func(_ context.Context, c DatabaseConfig) (*gorm.DB, error) {
					opened++
					require.Equal(t, cfg.StoreCenter.Database, c)
					if failAt == 1 {
						return nil, stop
					}
					return records, nil
				},
				CloseDatabase: func(db *gorm.DB) error { closed = append(closed, db); return nil },
				NewApplicationWithFeatures: func(_ context.Context, s *gorm.DB, f ApplicationFeatures, _ *coreconfig.Config, _ *logrus.Logger) (*http.Server, error) {
					require.Same(t, source, s)
					require.Same(t, resource, f.CommercialOwnerDB)
					if failAt == 2 {
						require.Same(t, records, f.StoreCenterDB)
					}
					return nil, stop
				},
			}
			require.ErrorIs(t, Run(context.Background(), cfg, logrus.New(), deps), stop)
			expected := []*gorm.DB{resource, source}
			if failAt == 2 {
				expected = append([]*gorm.DB{records}, expected...)
			}
			require.Equal(t, expected, closed)
			if failAt == 0 {
				require.Zero(t, opened)
			}
		})
	}
}
