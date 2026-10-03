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

	"task-processor/internal/app/httpapi"
	"task-processor/internal/app/productsourcing"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	assetstore "task-processor/internal/integration/persistence/product/asset"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	reviewstore "task-processor/internal/integration/persistence/product/review"
	"task-processor/internal/product/review"
	"task-processor/internal/storecenter"
)

func TestPrepareSampleReplaysWithoutDuplicatingFacts(t *testing.T) {
	dsn := os.Getenv("ISSUE36_TRIAL_TEST_DSN")
	if dsn == "" {
		t.Skip("requires isolated PostgreSQL ISSUE36_TRIAL_TEST_DSN")
	}
	root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	schema := "issue36_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, root.Exec("CREATE SCHEMA "+schema).Error)
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		if raw, e := db.DB(); e == nil {
			_ = raw.Close()
		}
		_ = root.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		if raw, e := root.DB(); e == nil {
			_ = raw.Close()
		}
	})
	require.NoError(t, httpapi.InstallProductReviewSchema(db))
	require.NoError(t, assetstore.AutoMigrate(db))
	require.NoError(t, storecenter.AutoMigrateStoreRepository(db))
	listingSchema, err := os.ReadFile("../listingrecordstore/schema.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(listingSchema)).Error)

	first, err := PrepareSample(context.Background(), db, "trial-org", "trial-user")
	require.NoError(t, err)
	require.NotEmpty(t, first.ProposalID)
	require.NotEmpty(t, first.RecordID)
	second, err := PrepareSample(context.Background(), db, "trial-org", "trial-user")
	require.NoError(t, err)
	require.Equal(t, first, second)
	authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	source, err := productsourcing.NewInternalProducer(db, setupAccess{organizationID: "trial-org", actorID: "trial-user"}, authorizer)
	require.NoError(t, err)
	reader, err := catalogstore.NewBoundedSnapshotReader(db, 2<<20)
	require.NoError(t, err)
	repository, err := reviewstore.NewRepository(db, func(tx *gorm.DB) (review.SourcePublicationReader, error) {
		return productsourcing.NewTransactionReader(tx)
	})
	require.NoError(t, err)
	service, err := review.NewCandidateService(reader, source, repository, authorizer)
	require.NoError(t, err)
	identity := authidentity.AuthenticatedIdentity{TenantID: "trial-org", EffectiveOrganizationID: "trial-org", HomeOrganizationID: "trial-org", UserID: "trial-user", Roles: []string{"listingkit_admin"}, TokenExpiresAt: time.Now().Add(time.Hour)}
	actor := authidentity.WithAuthenticatedIdentity(context.Background(), identity)
	accepted, err := service.Decide(actor, "accept-trial", first.ProposalID, review.DecisionInput{Action: "accept", ExpectedRevision: 1})
	require.NoError(t, err)
	applied, err := service.Apply(actor, "apply-trial", first.ProposalID, review.ApplyInput{ExpectedRevision: accepted.Revision})
	require.NoError(t, err)
	require.Equal(t, "applied", applied.State)
	third, err := PrepareSample(context.Background(), db, "trial-org", "trial-user")
	require.NoError(t, err)
	require.Equal(t, first, third)
	stillApplied, err := service.Get(actor, first.ProposalID)
	require.NoError(t, err)
	require.Equal(t, "applied", stillApplied.State)
	for _, table := range []string{"product_snapshot_versions", "product_title_proposals", "product_approved_assets", "workbench_stores", "listing_shein_records"} {
		var count int64
		require.NoError(t, db.Table(table).Count(&count).Error)
		want := int64(1)
		if table == "product_snapshot_versions" {
			want = 2
		}
		require.Equal(t, want, count, table)
	}
}
