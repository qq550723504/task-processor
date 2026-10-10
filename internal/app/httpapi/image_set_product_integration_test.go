package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/imageagent"
	"task-processor/internal/product/collection"
	"task-processor/internal/workbenchcontext"
)

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
