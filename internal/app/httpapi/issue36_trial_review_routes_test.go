package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	"task-processor/internal/workbenchcontext"
)

func TestIssue36ReviewOnlyRoutesKeepDecisionAndApplyWithoutGeneration(t *testing.T) {
	routes := productReviewOnlyRoutes(nil, nil)
	want := map[currentApplicationRoute]struct{}{
		{Method: http.MethodGet, Path: "/api/product/text-proposals"}:                         {},
		{Method: http.MethodGet, Path: "/api/product/text-proposals/:proposal_id"}:            {},
		{Method: http.MethodPost, Path: "/api/product/text-proposals/:proposal_id/decisions"}: {},
		{Method: http.MethodPost, Path: "/api/product/text-proposals/:proposal_id/apply"}:     {},
	}
	require.Len(t, routes, len(want))
	for _, route := range routes {
		key := currentApplicationRoute{Method: route.Method, Path: route.Path}
		_, admitted := want[key]
		require.True(t, admitted, "unexpected Review-only route: %s %s", route.Method, route.Path)
		delete(want, key)
		require.Equal(t, "product-review", route.Module)
		require.Equal(t, httproute.AuthPolicyVerifiedIdentity, route.AuthPolicy)
		if route.Method == http.MethodGet {
			require.Equal(t, httproute.OrganizationAccessPolicyCachedRead, route.OrganizationAccessPolicy)
			require.Equal(t, authz.PermissionLocalAgentWrite, route.Permission)
		} else {
			require.Equal(t, httproute.OrganizationAccessPolicyLiveWrite, route.OrganizationAccessPolicy)
			require.Equal(t, authz.PermissionLocalAgentWrite, route.Permission)
		}
	}
	require.Empty(t, want)
}

func TestIssue36ReviewOnlyApplicationUsesExistingReviewOwner(t *testing.T) {
	fixture := newTitleFixture(t)
	writer := fixture.server(t)
	created := titleCreate(t, writer, "seed-proposal")

	authorizer, err := authz.NewListingKitAuthorizer([]string{"operator"}, nil)
	require.NoError(t, err)
	resolver := workbenchcontext.NewResolver(fixture.grants, "project", "v1", nil)
	routes, err := buildLocalTrialReviewRoutes(fixture.db, resolver, authorizer)
	require.NoError(t, err)
	server := buildHTTPServerFromRoutesAtWithAuthDependencies("127.0.0.1", 0, routes, routeAuthDependencies{workbenchVerifier: titleVerifier{}, organizationResolver: resolver, authorizer: authorizer})
	trial := httptest.NewServer(server.Handler)
	t.Cleanup(trial.Close)
	code, raw, err := titleRequest(trial, http.MethodGet, titleBasePath+"?view=actionable", "operator", "B", "", "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, code, string(raw))
	code, _, err = titleRequest(trial, http.MethodPost, titleBasePath, "operator", "B", "new-proposal", `{"product_key":"product","base_version":1}`)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, code)
	accepted := titleDecision(t, trial, created, "accept", "admin", "", http.StatusOK)
	applied := titleApply(t, trial, accepted, "apply-seed-proposal", http.StatusOK)
	require.Equal(t, "applied", applied.State)
	require.NotNil(t, applied.Receipt)
	require.EqualValues(t, 2, applied.Receipt.ProductVersion)
}
