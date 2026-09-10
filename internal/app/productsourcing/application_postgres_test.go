package productsourcing

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

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/product/sourcing"
)

func TestInternalProducerCreatesProductFromBusinessCallOnEmptyPostgres(t *testing.T) {
	dsn := os.Getenv("ISSUE378_TEST_DSN")
	if dsn == "" {
		t.Skip("result=SKIP: ISSUE378_TEST_DSN must target task-isolated PostgreSQL")
	}
	root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	schema := "issue378_app_" + strings.ReplaceAll(uuid.NewString(), "-", "_")
	require.NoError(t, root.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() { require.NoError(t, root.Exec("DROP SCHEMA "+schema+" CASCADE").Error) })
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	// Constructor performs no implicit DDL. The task-owned empty database uses
	// the explicit initializer before the first business call.
	permissions, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	liveCalls := 0
	roles := []string{"listingkit_operator"}
	producer, err := NewInternalProducer(db, liveRolesFunc(func(_ context.Context, org, actor string) ([]string, error) {
		liveCalls++
		require.Equal(t, "org-a", org)
		require.Equal(t, "actor-a", actor)
		return roles, nil
	}), permissions)
	require.NoError(t, err)
	var tables int64
	require.NoError(t, db.Raw("SELECT count(*) FROM information_schema.tables WHERE table_schema = ?", schema).Scan(&tables).Error)
	require.Zero(t, tables, "ordinary composition must not install schema")
	require.NoError(t, InstallSchema(db))
	var tableNames []string
	require.NoError(t, db.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = ? ORDER BY table_name", schema).Scan(&tableNames).Error)
	require.ElementsMatch(t, []string{"product_snapshot_heads", "product_snapshot_versions", "product_source_publication_receipts", "product_source_publications"}, tableNames)

	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{
		TenantID: "org-a", EffectiveOrganizationID: "org-a", UserID: "actor-a",
		TokenExpiresAt: time.Now().Add(time.Hour),
	})
	base := uint64(0)
	command := sourcing.PublicationCommand{
		PublicationID: "pub-app-1", Producer: sourcing.ProducerDescriptor{Kind: "controlled_snapshot", Version: "v1"},
		ProductKey: "product-app-1", ExpectedBaseVersion: &base,
		Envelope: sourcing.SourceEnvelope{
			Identity:         sourcing.SourceIdentity{SourceType: sourcing.SourceTypeManualImport, SourcePlatform: "controlled", SourceID: "source-app-1", SourceVersion: "v1"},
			RawReference:     sourcing.RawSourceReference{ReferenceType: "snapshot", ReferenceID: "raw-app-1", Checksum: "sha256:abc"},
			ProductCandidate: sourcing.ProductCandidate{Title: "Business-created bottle"},
			AssetCandidates:  []sourcing.AssetCandidate{{SourceID: "image-1", URL: "https://example.test/image-1.png", MediaType: "image", Role: "primary"}},
			MissingFacts:     []sourcing.MissingFact{{Field: "weight", Reason: "not supplied"}},
		},
	}
	receipt, err := producer.Publish(ctx, command)
	require.NoError(t, err)
	require.Equal(t, uint64(1), receipt.CatalogVersion)
	persisted, err := producer.Read(ctx, "pub-app-1")
	require.NoError(t, err)
	require.Equal(t, "Business-created bottle", persisted.Snapshot.Title)
	require.Equal(t, "weight", persisted.Envelope.MissingFacts[0].Field)
	require.Len(t, persisted.Snapshot.Images, 1, "source image remains a Catalog candidate")
	require.Equal(t, 2, liveCalls, "publish and read must independently refresh authorization")

	roles = []string{"listingkit_viewer"}
	_, err = producer.Publish(ctx, command)
	require.ErrorIs(t, err, sourcing.ErrPublicationForbidden)
	_, err = producer.Read(ctx, "pub-app-1")
	require.ErrorIs(t, err, sourcing.ErrPublicationForbidden)
	require.Equal(t, 4, liveCalls, "replay and read must observe live revocation")
}
