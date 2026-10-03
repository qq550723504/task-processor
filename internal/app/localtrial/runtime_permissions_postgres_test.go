package localtrial

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	assetstore "task-processor/internal/integration/persistence/product/asset"
	"task-processor/internal/storecenter"
)

func TestVerifyRuntimePermissionsRejectsMissingOrExcessRights(t *testing.T) {
	dsn := os.Getenv("ISSUE36_TRIAL_TEST_DSN")
	if dsn == "" {
		t.Skip("requires isolated PostgreSQL ISSUE36_TRIAL_TEST_DSN")
	}
	require.Contains(t, dsn, "dbname=postgres")
	root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	database, role := "issue36perm_"+suffix, "issue36role_"+suffix
	require.NoError(t, root.Exec("CREATE ROLE "+role+" LOGIN PASSWORD 'test_only'").Error)
	require.NoError(t, root.Exec("CREATE DATABASE "+database).Error)
	t.Cleanup(func() {
		_ = root.Exec("DROP DATABASE " + database + " WITH (FORCE)").Error
		_ = root.Exec("DROP ROLE " + role).Error
		if raw, e := root.DB(); e == nil {
			_ = raw.Close()
		}
	})
	ownerDSN := strings.Replace(dsn, "dbname=postgres", "dbname="+database, 1)
	owner, err := gorm.Open(postgres.Open(ownerDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		if raw, e := owner.DB(); e == nil {
			_ = raw.Close()
		}
	})
	require.NoError(t, installTestReviewSchema(owner))
	require.NoError(t, assetstore.AutoMigrate(owner))
	require.NoError(t, storecenter.AutoMigrateStoreRepository(owner))
	listingSchema, err := os.ReadFile("../listingrecordstore/schema.sql")
	require.NoError(t, err)
	require.NoError(t, owner.Exec(string(listingSchema)).Error)
	require.NoError(t, owner.Exec("GRANT CONNECT ON DATABASE "+database+" TO "+role).Error)
	require.NoError(t, owner.Exec("GRANT USAGE ON SCHEMA public TO "+role).Error)
	require.NoError(t, owner.Exec("GRANT SELECT ON public.workbench_stores TO "+role).Error)
	for _, table := range trialWritableTables {
		require.NoError(t, owner.Exec("GRANT SELECT, INSERT, UPDATE ON public."+table+" TO "+role).Error)
	}
	runtimeDSN := strings.Replace(ownerDSN, "user=postgres", "user="+role, 1)
	runtimeDSN = strings.Replace(runtimeDSN, "password=issue36_test_only", "password=test_only", 1)
	runtime, err := gorm.Open(postgres.Open(runtimeDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		if raw, e := runtime.DB(); e == nil {
			_ = raw.Close()
		}
	})
	require.NoError(t, VerifyRuntimePermissions(context.Background(), runtime, role, database))
	require.NoError(t, owner.Exec("REVOKE SELECT ON public.listing_shein_records FROM "+role).Error)
	require.Error(t, VerifyRuntimePermissions(context.Background(), runtime, role, database))
	require.NoError(t, owner.Exec("GRANT SELECT ON public.listing_shein_records TO "+role).Error)
	require.NoError(t, owner.Exec("GRANT CREATE ON SCHEMA public TO "+role).Error)
	require.Error(t, VerifyRuntimePermissions(context.Background(), runtime, role, database))
}
