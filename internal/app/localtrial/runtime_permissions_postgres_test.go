package localtrial

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/app/productsourcing"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	assetstore "task-processor/internal/integration/persistence/product/asset"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	reviewstore "task-processor/internal/integration/persistence/product/review"
	"task-processor/internal/product/review"
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
	sample, err := PrepareSample(context.Background(), owner, "trial-org", "trial-user")
	require.NoError(t, err)
	authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	source, err := productsourcing.NewInternalProducer(runtime, setupAccess{organizationID: "trial-org", actorID: "trial-user"}, authorizer)
	require.NoError(t, err)
	reader, err := catalogstore.NewBoundedSnapshotReader(runtime, 2<<20)
	require.NoError(t, err)
	repository, err := reviewstore.NewRepository(runtime, func(tx *gorm.DB) (review.SourcePublicationReader, error) {
		return productsourcing.NewTransactionReader(tx)
	})
	require.NoError(t, err)
	service, err := review.NewCandidateService(reader, source, repository, authorizer)
	require.NoError(t, err)
	actor := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{
		TenantID: "trial-org", EffectiveOrganizationID: "trial-org", HomeOrganizationID: "trial-org",
		UserID: "trial-user", Roles: []string{"listingkit_admin"}, TokenExpiresAt: time.Now().Add(time.Hour),
	})
	accepted, err := service.Decide(actor, "accept-narrow-runtime", sample.ProposalID, review.DecisionInput{Action: "accept", ExpectedRevision: 1})
	require.NoError(t, err)
	applied, err := service.Apply(actor, "apply-narrow-runtime", sample.ProposalID, review.ApplyInput{ExpectedRevision: accepted.Revision})
	require.NoError(t, err)
	require.Equal(t, "applied", applied.State)
	var recordCount int64
	require.NoError(t, runtime.Table("listing_shein_records").Where("organization_id = ? AND id = ?", "trial-org", sample.RecordID).Count(&recordCount).Error)
	require.EqualValues(t, 1, recordCount)
	require.NoError(t, owner.Exec("REVOKE SELECT ON public.listing_shein_records FROM "+role).Error)
	require.Error(t, VerifyRuntimePermissions(context.Background(), runtime, role, database))
	require.NoError(t, owner.Exec("GRANT SELECT ON public.listing_shein_records TO "+role).Error)
	require.NoError(t, owner.Exec("GRANT CREATE ON SCHEMA public TO "+role).Error)
	require.Error(t, VerifyRuntimePermissions(context.Background(), runtime, role, database))
}
