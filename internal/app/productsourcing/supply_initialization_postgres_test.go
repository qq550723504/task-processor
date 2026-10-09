package productsourcing

import (
	"context"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	acquisitionstore "task-processor/internal/integration/persistence/product/acquisition"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	"testing"
	"time"
)

func TestSupplyInitializationAdmitsExactProductPermissions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("product605"), tcpostgres.WithUsername("product_owner"), tcpostgres.WithPassword("isolated_product"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	owner, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := owner.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = pool.Close() })
	require.NoError(t, InstallAcquisitionSchema(owner))
	require.NoError(t, collectionstore.InstallSchema(owner))
	require.NoError(t, InstallSupplyChainSchema(owner))
	require.NoError(t, owner.Exec(`CREATE ROLE source_acquisition_runtime LOGIN PASSWORD 'isolated_supply' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS`).Error)
	capabilities := acquisitionstore.RuntimeCapabilities{Collections: true, SupplyChain: true}
	require.NoError(t, acquisitionstore.GrantRuntimePermissions(ctx, owner, capabilities))
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	parsed.User = url.UserPassword(acquisitionstore.RuntimeRole, "isolated_supply")
	runtimeDB, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	runtimePool, err := runtimeDB.DB()
	require.NoError(t, err)
	runtimePool.SetMaxOpenConns(2)
	t.Cleanup(func() { _ = runtimePool.Close() })
	require.NoError(t, acquisitionstore.VerifyRuntimePermissions(ctx, runtimeDB, capabilities))
	require.Error(t, acquisitionstore.VerifyRuntimePermissions(ctx, runtimeDB, acquisitionstore.RuntimeCapabilities{Collections: true}), "all Product consumers must admit the supply capability footprint")
	require.NoError(t, owner.Exec(`GRANT UPDATE ON product_title_proposals TO source_acquisition_runtime`).Error)
	require.Error(t, acquisitionstore.VerifyRuntimePermissions(ctx, runtimeDB, capabilities), "Review writes stay with the separate Review pool")
}
