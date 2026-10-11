//go:build integration

package productsourcing

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	"os"
	"path/filepath"
	acquisitionstore "task-processor/internal/integration/persistence/product/acquisition"
	"testing"
	"time"
)

func TestDataServicesFreshInstallerKeepsProductRolesNarrow(t *testing.T) {
	t.Run("data services", func(t *testing.T) { verifyDataServicesFreshInstaller(t, false) })
	t.Run("data services with supply market", func(t *testing.T) { verifyDataServicesFreshInstaller(t, true) })
}

func verifyDataServicesFreshInstaller(t *testing.T, supplyMarket bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("product_runtime"), tcpostgres.WithUsername("test_owner"), tcpostgres.WithPassword("isolated-test-only"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	address, err := url.Parse(dsn)
	require.NoError(t, err)
	open := func(address string, max int) *gorm.DB {
		db, err := gorm.Open(postgres.Open(address), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		require.NoError(t, err)
		pool, err := db.DB()
		require.NoError(t, err)
		pool.SetMaxOpenConns(max)
		t.Cleanup(func() { require.NoError(t, pool.Close()) })
		return db
	}
	owner := open(dsn, 2)
	require.NoError(t, owner.Exec(`CREATE ROLE source_acquisition_runtime LOGIN PASSWORD 'runtime-fixture'; CREATE ROLE data_services_runtime LOGIN PASSWORD 'runtime-fixture'`).Error)
	port, err := container.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "install.json")
	manifest := map[string]any{"schemaVersion": 1, "collections": true, "dataServices": true, "supplyMarket": supplyMarket, "database": map[string]any{"host": "127.0.0.1", "port": int(port.Num()), "database": "product_runtime", "user": "test_owner", "password": "isolated-test-only"}}
	raw, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0600))
	require.NoError(t, InitializeAcquisitionDatabase(ctx, path, "product_runtime"))
	require.Error(t, InitializeAcquisitionDatabase(ctx, path, "product_runtime"), "serving/retained installs cannot repair or migrate Product")
	sourceURL := *address
	sourceURL.User = url.UserPassword(acquisitionstore.RuntimeRole, "runtime-fixture")
	dataURL := *address
	dataURL.User = url.UserPassword(acquisitionstore.DataServicesRuntimeRole, "runtime-fixture")
	source := open(sourceURL.String(), 4)
	data := open(dataURL.String(), 4)
	require.NoError(t, acquisitionstore.VerifyRuntimePermissions(ctx, source, acquisitionstore.RuntimeCapabilities{Collections: true, SupplyMarket: supplyMarket}))
	require.NoError(t, acquisitionstore.VerifyDataServicesRuntimePermissions(ctx, data))
	require.Error(t, source.Exec("SELECT id FROM data_service_credentials LIMIT 0").Error)
	require.Error(t, data.Exec("SELECT operation_id FROM product_acquisition_operations LIMIT 0").Error)
	require.Error(t, data.Exec("CREATE TABLE forbidden_runtime_ddl(id int)").Error)
	require.Error(t, data.Exec("DELETE FROM data_service_commands WHERE false").Error)
	require.NoError(t, owner.Exec("GRANT SELECT (operation_id) ON product_acquisition_operations TO data_services_runtime").Error)
	require.Error(t, acquisitionstore.VerifyDataServicesRuntimePermissions(ctx, data), "extra column privileges fail closed")
	require.NoError(t, owner.Exec("REVOKE SELECT (operation_id) ON product_acquisition_operations FROM data_services_runtime").Error)
	require.NoError(t, acquisitionstore.VerifyDataServicesRuntimePermissions(ctx, data))
	require.NoError(t, owner.Exec("GRANT test_owner TO data_services_runtime").Error)
	require.Error(t, acquisitionstore.VerifyDataServicesRuntimePermissions(ctx, data), "owner membership fails closed")
}
