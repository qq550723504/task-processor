package httpapi

import (
	"context"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	resourceadapter "task-processor/internal/integration/orgresource"
	commercialstore "task-processor/internal/integration/persistence/commercialbilling"
	moneystore "task-processor/internal/integration/persistence/money"
	"testing"
)

func TestUnifiedResourcePurchasesDoNotRequireDisabledPaymentDirectory(t *testing.T) {
	open := func(name string) *gorm.DB {
		db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), name)), &gorm.Config{})
		require.NoError(t, err)
		sql, err := db.DB()
		require.NoError(t, err)
		t.Cleanup(func() { sql.Close() })
		return db
	}
	resource, money := open("resource.db"), open("money.db")
	require.NoError(t, commercialstore.AutoMigrate(resource))
	require.NoError(t, resourceadapter.AutoMigrate(resource))
	require.NoError(t, moneystore.AutoMigrate(money))
	cfg := &config.Config{}
	module, err := buildCommercialBillingModule(context.Background(), resource, money, authz.DefaultListingKitAuthorizer(), cfg)
	require.NoError(t, err)
	require.NotNil(t, module)
	// A payment channel still needs its own real runtime prerequisites.
	cfg.WalletTopUp.Alipay.Enabled = true
	_, err = buildCommercialBillingModule(context.Background(), resource, money, authz.DefaultListingKitAuthorizer(), cfg)
	require.Error(t, err)
}
