package productsourcing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/authz"
	acquisitionpersistence "task-processor/internal/integration/persistence/product/acquisition"
	"task-processor/internal/product/sourcing"
)

func acquisitionPermissionDependencies(t *testing.T) (*authz.ListingKitAuthorizer, sourcing.LiveOrganizationAccess) {
	t.Helper()
	permissions, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	return permissions, liveRolesFunc(func(context.Context, string, string) ([]string, error) { return []string{"listingkit_operator"}, nil })
}

func TestAcquisitionRuntimePermissionsRejectEscalationAndMissingPrivileges(t *testing.T) {
	db := acquisitionDatabase(t)
	ctx := context.Background()
	require.NoError(t, InstallAcquisitionSchema(db))
	// This fixture role belongs only to the task-labeled PostgreSQL container.
	// The production initializer never creates or changes cluster credentials.
	sum := sha256.Sum256([]byte(os.Getenv("ISSUE398_TEST_DSN")))
	password := hex.EncodeToString(sum[:])
	require.NoError(t, db.Exec(`DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='source_acquisition_runtime') THEN CREATE ROLE source_acquisition_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS; END IF; END $$`).Error)
	require.NoError(t, db.Exec("ALTER ROLE source_acquisition_runtime PASSWORD '"+password+"'").Error)
	require.NoError(t, acquisitionpersistence.GrantRuntimePermissions(ctx, db))
	var name string
	require.NoError(t, db.Raw("SELECT current_database()").Scan(&name).Error)
	dsn := os.Getenv("ISSUE398_TEST_DSN") + " dbname=" + name + " user=source_acquisition_runtime password=" + password
	runtimeDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := runtimeDB.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(8)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	require.NoError(t, acquisitionpersistence.VerifyRuntimePermissions(ctx, runtimeDB))
	require.Error(t, acquisitionpersistence.VerifyRuntimePermissions(ctx, db), "owner role is not a runtime")
	tests := []struct{ name, grant, revoke string }{
		{"delete", "GRANT DELETE ON product_acquisition_operations TO source_acquisition_runtime", "REVOKE DELETE ON product_acquisition_operations FROM source_acquisition_runtime"},
		{"column_update_on_evidence", "GRANT UPDATE(envelope_hash) ON product_source_publications TO source_acquisition_runtime", "REVOKE UPDATE(envelope_hash) ON product_source_publications FROM source_acquisition_runtime"},
		{"column_update_on_catalog_versions", "GRANT UPDATE(payload_hash) ON product_snapshot_versions TO source_acquisition_runtime", "REVOKE UPDATE(payload_hash) ON product_snapshot_versions FROM source_acquisition_runtime"},
		{"schema_create", "GRANT CREATE ON SCHEMA public TO source_acquisition_runtime", "REVOKE CREATE ON SCHEMA public FROM source_acquisition_runtime"},
		{"database_temp", "GRANT TEMPORARY ON DATABASE " + name + " TO source_acquisition_runtime", "REVOKE TEMPORARY ON DATABASE " + name + " FROM source_acquisition_runtime"},
		{"missing_insert", "REVOKE INSERT ON product_source_publication_receipts FROM source_acquisition_runtime", "GRANT INSERT ON product_source_publication_receipts TO source_acquisition_runtime"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, db.Exec(tc.grant).Error)
			require.Error(t, acquisitionpersistence.VerifyRuntimePermissions(ctx, runtimeDB))
			require.NoError(t, db.Exec(tc.revoke).Error)
			require.NoError(t, acquisitionpersistence.VerifyRuntimePermissions(ctx, runtimeDB))
		})
	}
	require.NoError(t, db.Exec("CREATE TABLE public.unrelated_secret(value text)").Error)
	require.NoError(t, db.Exec("GRANT SELECT(value) ON public.unrelated_secret TO source_acquisition_runtime").Error)
	require.Error(t, acquisitionpersistence.VerifyRuntimePermissions(ctx, runtimeDB))
	require.NoError(t, db.Exec("REVOKE SELECT(value) ON public.unrelated_secret FROM source_acquisition_runtime").Error)
	require.NoError(t, acquisitionpersistence.VerifyRuntimePermissions(ctx, runtimeDB))
	pool.SetMaxOpenConns(9)
	require.Error(t, acquisitionpersistence.VerifyRuntimePermissions(ctx, runtimeDB))
	pool.SetMaxOpenConns(8)
	// The real acquisition chain must work using only the admitted five-table
	// grants, not merely pass a catalog privilege query.
	provider, count, _ := acquisitionFixture(t)
	permissions, live := acquisitionPermissionDependencies(t)
	service, err := NewPublicAcquisition(ctx, runtimeDB, live, permissions, provider)
	require.NoError(t, err)
	result, err := service.Acquire(acquisitionIdentity("org-runtime", "actor"), uuid.NewString(), "981645030344")
	require.NoError(t, err)
	require.NotNil(t, result.Publication)
	require.EqualValues(t, 1, result.Publication.Receipt.CatalogVersion)
	require.EqualValues(t, 1, count.Load())
	require.Error(t, runtimeDB.Exec("DELETE FROM product_acquisition_operations").Error)
	require.Error(t, runtimeDB.Exec("CREATE TABLE forbidden_runtime_table(id int)").Error)
}
