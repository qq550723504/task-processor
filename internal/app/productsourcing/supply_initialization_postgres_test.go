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
	"strconv"
	acquisitionstore "task-processor/internal/integration/persistence/product/acquisition"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	"testing"
	"time"
)

func TestImageSetInitializationDoesNotGrantCollectionWrites(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("image612"), tcpostgres.WithUsername("product_owner"), tcpostgres.WithPassword("isolated_product"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	owner, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := owner.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = pool.Close() })
	require.NoError(t, owner.Exec(`CREATE ROLE source_acquisition_runtime LOGIN PASSWORD 'isolated_image' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS`).Error)
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	port, err := strconv.Atoi(parsed.Port())
	require.NoError(t, err)
	manifest := filepath.Join(t.TempDir(), "image-init.json")
	raw, err := json.Marshal(map[string]any{"schemaVersion": 1, "imageSets": true, "database": map[string]any{"host": "127.0.0.1", "port": port, "user": "product_owner", "password": "isolated_product", "database": "image612"}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(manifest, raw, 0600))
	require.NoError(t, InitializeAcquisitionDatabase(ctx, manifest, "image612"))
	require.True(t, owner.Migrator().HasTable("product_title_proposals"))
	require.False(t, owner.Migrator().HasTable("product_collection_batches"))
	parsed.User = url.UserPassword(acquisitionstore.RuntimeRole, "isolated_image")
	runtimeDB, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	runtimePool, err := runtimeDB.DB()
	require.NoError(t, err)
	runtimePool.SetMaxOpenConns(2)
	t.Cleanup(func() { _ = runtimePool.Close() })
	require.NoError(t, acquisitionstore.VerifyRuntimePermissions(ctx, runtimeDB, acquisitionstore.RuntimeCapabilities{ImageSets: true}))
	var allowed bool
	require.NoError(t, runtimeDB.Raw(`SELECT has_table_privilege(current_user,'product_title_proposals','SELECT') AND NOT has_table_privilege(current_user,'product_title_proposals','INSERT,UPDATE,DELETE') AND NOT has_table_privilege(current_user,'product_title_operations','SELECT,INSERT,UPDATE,DELETE')`).Scan(&allowed).Error)
	require.True(t, allowed)
}

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
	require.NoError(t, owner.Exec(`REVOKE UPDATE ON product_title_proposals FROM source_acquisition_runtime`).Error)
	images := acquisitionstore.RuntimeCapabilities{Collections: true, ImageSets: true}
	require.NoError(t, acquisitionstore.GrantRuntimePermissions(ctx, owner, images))
	require.NoError(t, acquisitionstore.VerifyRuntimePermissions(ctx, runtimeDB, images))
	require.Error(t, acquisitionstore.VerifyRuntimePermissions(ctx, runtimeDB, acquisitionstore.RuntimeCapabilities{Collections: true}), "disabled image capability refuses its additional read")
	var allowed bool
	require.NoError(t, runtimeDB.Raw(`SELECT has_table_privilege(current_user,'product_title_proposals','SELECT') AND NOT has_table_privilege(current_user,'product_title_proposals','INSERT,UPDATE,DELETE') AND NOT has_table_privilege(current_user,'product_title_operations','SELECT,INSERT,UPDATE,DELETE') AND NOT has_table_privilege(current_user,'listing_preparations','SELECT,INSERT,UPDATE,DELETE')`).Scan(&allowed).Error)
	require.True(t, allowed, "images admit only the existing AppliedPublicationLookup read")
	require.NoError(t, owner.Exec(`GRANT SELECT ON product_title_operations TO source_acquisition_runtime`).Error)
	require.Error(t, acquisitionstore.VerifyRuntimePermissions(ctx, runtimeDB, images))
	require.NoError(t, owner.Exec(`REVOKE SELECT ON product_title_operations FROM source_acquisition_runtime`).Error)
	require.NoError(t, owner.Exec(`GRANT UPDATE(payload) ON product_title_proposals TO source_acquisition_runtime`).Error)
	require.Error(t, acquisitionstore.VerifyRuntimePermissions(ctx, runtimeDB, images))
	require.NoError(t, owner.Exec(`REVOKE UPDATE(payload) ON product_title_proposals FROM source_acquisition_runtime`).Error)
	require.NoError(t, acquisitionstore.VerifyRuntimePermissions(ctx, runtimeDB, images))
}
