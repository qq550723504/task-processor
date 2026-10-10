package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/imageagent"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/review"
	"task-processor/internal/workbenchcontext"
)

func TestFullImageAcquisitionOptionsDoNotRequireCollections(t *testing.T) {
	capability, options, err := collectionAcquisitionOptions([]bool{false, false, true})
	require.NoError(t, err)
	require.True(t, capability.ImageSets)
	require.False(t, capability.Collections)
	require.Empty(t, options)
	_, _, err = collectionAcquisitionOptions([]bool{false, true, true})
	require.Error(t, err, "Supply still requires Collections")
}

func TestFullImageProductReaderNeedsOnlySnapshotAndAppliedTitleReads(t *testing.T) {
	f := newTitleFixture(t)
	var schema string
	require.NoError(t, f.db.Raw("SELECT current_schema()").Scan(&schema).Error)
	role := "image_read_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, f.db.Exec("CREATE ROLE "+role+" LOGIN PASSWORD 'controlled-image-reader' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS").Error)
	require.NoError(t, f.db.Exec("GRANT USAGE ON SCHEMA "+schema+" TO "+role).Error)
	require.NoError(t, f.db.Exec("GRANT SELECT ON product_snapshot_versions,product_snapshot_heads,product_title_proposals TO "+role).Error)
	db, err := gorm.Open(postgres.Open(os.Getenv("ISSUE382_TEST_DSN")+" search_path="+schema+" user="+role+" password=controlled-image-reader"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(2)
	t.Cleanup(func() {
		_ = pool.Close()
		_ = f.db.Exec("DROP OWNED BY " + role).Error
		_ = f.db.Exec("DROP ROLE " + role).Error
	})
	require.False(t, productReviewSchemaReady(db), "a read consumer must not need Review operation access")
	permissions, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	cfg := &config.Config{}
	cfg.ListingKit.Zitadel.TenantDirectoryToken = "controlled-current-reader"
	products, _, err := buildFullImageProducts(db, routeAuthDependencies{organizationResolver: workbenchcontext.NewResolver(f.grants, "project", "v1", nil)}, permissions, cfg)
	require.NoError(t, err)
	id := imageagent.ExecutionIdentity{TenantID: "B", UserID: "operator", MemberID: "member-B-operator"}
	result, err := products.Snapshots.GetSnapshot(context.Background(), f.base.Identity, f.base.Version)
	require.NoError(t, err)
	require.Equal(t, f.base, result)
	_, err = products.Applied.ReadAppliedPublication(context.Background(), review.Scope{Org: "B", Actor: "operator"}, f.base.Identity.ProductKey, f.base.Version, f.base.PublicationID)
	require.ErrorIs(t, err, review.ErrNotFound, "the scoped lookup executes with SELECT only")
	id.TenantID = "A"
	_, err = products.ReadEffectiveImageProduct(context.Background(), id, f.base, f.base.Version, "")
	require.ErrorIs(t, err, imageagent.ErrIdentityRequired)
	require.Error(t, db.Exec("UPDATE product_title_proposals SET state=state").Error)
	require.Error(t, db.Exec("SELECT * FROM product_title_operations").Error)
}

func TestFullImageGenericUsesCurrentProductAndExactLiveSourceMembership(t *testing.T) {
	f := newTitleFixture(t)
	var revoked atomic.Bool
	iam := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer controlled-current-reader", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		member := "member-B-admin"
		if revoked.Load() {
			member = "replacement-member"
		}
		fmt.Fprintf(w, `{"pagination":{"totalResult":"1"},"authorizations":[{"id":%q,"project":{"id":"project"},"organization":{"id":"B"},"user":{"id":"admin"},"state":"STATE_ACTIVE","roles":[{"key":"listingkit_admin"}]}]}`, member)
	}))
	defer iam.Close()
	permissions, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	cfg := &config.Config{}
	cfg.ListingKit.Zitadel.AuthorizationAPIURL = iam.URL
	cfg.ListingKit.Zitadel.ProjectID = "project"
	cfg.ListingKit.Zitadel.TenantDirectoryToken = "controlled-current-reader"
	resolver := workbenchcontext.NewResolver(f.grants, "project", "v1", nil)
	products, live, err := buildFullImageProducts(f.db, routeAuthDependencies{organizationResolver: resolver}, permissions, cfg)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	exact, err := products.Snapshots.GetSnapshot(ctx, f.base.Identity, f.base.Version)
	require.NoError(t, err)
	require.Equal(t, f.base, exact)
	id := imageagent.ExecutionIdentity{TenantID: "B", UserID: "admin", MemberID: "member-B-admin"}
	effective, err := products.ReadEffectiveImageProduct(ctx, id, exact, exact.Version, "")
	require.NoError(t, err)
	require.Equal(t, exact, effective)
	id.TenantID = "A"
	_, err = products.ReadEffectiveImageProduct(ctx, id, exact, exact.Version, "")
	require.ErrorIs(t, err, imageagent.ErrIdentityRequired)
	scope := collection.Scope{OrganizationID: "B", ActorID: "admin", MemberID: "member-B-admin"}
	require.NoError(t, live.AuthorizeImageSource(ctx, scope))
	revoked.Store(true)
	require.ErrorIs(t, live.AuthorizeImageSource(ctx, scope), collection.ErrForbidden)
	cfg.ListingKit.Zitadel.TenantDirectoryToken = ""
	_, _, err = buildFullImageProducts(f.db, routeAuthDependencies{organizationResolver: resolver}, permissions, cfg)
	require.ErrorIs(t, err, imageagent.ErrCommandBlocked)
}
