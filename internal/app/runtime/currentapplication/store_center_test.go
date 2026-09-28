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

func storeRuntimeConfig() *Config {
	c := runtimeTestConfig()
	owner := c.CommercialDatabase
	owner.User = "commercial_owner_runtime"
	c.CommercialOwnerDatabase = &owner
	store := c.SourceAccountDatabase
	store.User, store.Database = "store_center_runtime", "stores"
	quota := owner
	quota.User = "store_quota_runtime"
	c.StoreCenter = &StoreCenterConfig{Enabled: true, Database: store, QuotaDatabase: quota}
	return c
}

func TestStoreCenterRequiresCanonicalOwnerAndNarrowIndependentPools(t *testing.T) {
	require.NoError(t, storeRuntimeConfig().validate())
	for name, mutate := range map[string]func(*Config){
		"missing owner":          func(c *Config) { c.CommercialOwnerDatabase = nil },
		"wrong quota target":     func(c *Config) { c.StoreCenter.QuotaDatabase.Database = "other" },
		"borrow wide quota role": func(c *Config) { c.StoreCenter.QuotaDatabase.User = "commercial_owner_runtime" },
		"borrow wide store role": func(c *Config) { c.StoreCenter.Database.User = "commercial_runtime" },
		"shared record database": func(c *Config) { c.StoreCenter.Database.Database = c.SourceAccountDatabase.Database },
		"unbounded quota pool":   func(c *Config) { c.StoreCenter.QuotaDatabase.MaxConnections = 9 },
		"unbounded record pool":  func(c *Config) { c.StoreCenter.Database.MaxConnections = 9 },
	} {
		t.Run(name, func(t *testing.T) { c := storeRuntimeConfig(); mutate(c); require.Error(t, c.validate()) })
	}
	c := runtimeTestConfig()
	c.StoreCenter = &StoreCenterConfig{Enabled: false}
	require.NoError(t, c.validate(), "disabled feature requires no databases")
}

func TestStoreCenterRuntimeClosesPoolsOnPartialStartup(t *testing.T) {
	for failAt := 0; failAt <= 3; failAt++ {
		t.Run(string(rune('0'+failAt)), func(t *testing.T) {
			c := storeRuntimeConfig()
			c.StoreCenter.Enabled = failAt != 0
			pools := []*gorm.DB{{}, {}, {}, {}, {}}
			var closed []*gorm.DB
			opened := 0
			stop := errors.New("isolated stop before listener")
			deps := Dependencies{
				IdentityPreflight:   func(context.Context, IdentityConfig) error { return nil },
				OpenSourceAccount:   func(context.Context, DatabaseConfig) (*gorm.DB, error) { return pools[0], nil },
				OpenCommercial:      func(context.Context, DatabaseConfig) (*gorm.DB, error) { return pools[1], nil },
				OpenCommercialOwner: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return pools[2], nil },
				OpenStoreCenter: func(_ context.Context, d DatabaseConfig) (*gorm.DB, error) {
					opened++
					require.Equal(t, c.StoreCenter.Database, d)
					if failAt == 1 {
						return nil, stop
					}
					return pools[3], nil
				},
				OpenStoreQuota: func(_ context.Context, d DatabaseConfig) (*gorm.DB, error) {
					opened++
					require.Equal(t, c.StoreCenter.QuotaDatabase, d)
					if failAt == 2 {
						return nil, stop
					}
					return pools[4], nil
				},
				CloseDatabase: func(d *gorm.DB) error { closed = append(closed, d); return nil },
				NewApplicationWithFeatures: func(_ context.Context, _, _ *gorm.DB, f ApplicationFeatures, _ *coreconfig.Config, _ *logrus.Logger) (*http.Server, error) {
					if failAt == 3 {
						require.Same(t, pools[3], f.StoreCenterDB)
						require.Same(t, pools[4], f.StoreQuotaDB)
					}
					return nil, stop
				},
			}
			require.ErrorIs(t, Run(context.Background(), c, logrus.New(), deps), stop)
			expected := []*gorm.DB{pools[2], pools[1], pools[0]}
			if failAt == 2 {
				expected = append([]*gorm.DB{pools[3]}, expected...)
			}
			if failAt == 3 {
				expected = append([]*gorm.DB{pools[4], pools[3]}, expected...)
			}
			require.Equal(t, expected, closed)
			if failAt == 0 {
				require.Zero(t, opened)
			}
		})
	}
}
