package knowledgeauth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	"task-processor/internal/integration/commercetoolauth"
	k "task-processor/internal/knowledge"
	"task-processor/internal/workbenchcontext"
)

type grants struct {
	roles   []string
	revoked bool
	err     error
	calls   int
}

func (g *grants) ListOwnProjectAuthorizations(context.Context, string, string, string) ([]authidentity.OrganizationGrant, error) {
	g.calls++
	if g.err != nil {
		return nil, g.err
	}
	if g.revoked {
		return nil, nil
	}
	return []authidentity.OrganizationGrant{{OrganizationID: "org", ProjectID: "project", Roles: g.roles}}, nil
}
func TestKnowledgeAuthorizerUsesLiveGrantAndExistingRoles(t *testing.T) {
	g := &grants{roles: []string{"listingkit_operator"}}
	resolver := workbenchcontext.NewResolver(workbenchcontext.NewGrantResolver(g, nil), "project", "v1", nil)
	policy, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	a, err := NewAuthorizer(resolver, policy)
	require.NoError(t, err)
	request := commercetoolauth.OrganizationRequest{Identity: authidentity.AuthenticatedIdentity{UserID: "actor", HomeOrganizationID: "org", TokenExpiresAt: time.Now().Add(time.Hour)}, BearerToken: "synthetic-test-token", RequestedOrganizationID: "org"}
	ctx := commercetoolauth.WithOrganizationRequest(context.Background(), request)
	// Warm the actual grant cache, then change roles/revoke without invalidating.
	_, err = resolver.Resolve(ctx, httproute.OrganizationAccessPolicyCachedRead, workbenchcontext.ResolveInput{Identity: request.Identity, BearerToken: request.BearerToken, RequestedOrganizationID: "org"})
	require.NoError(t, err)
	for _, role := range []string{"listingkit_operator", "listingkit_admin", "listingkit_viewer"} {
		g.roles = []string{role}
		scope, err := a.AuthorizeKnowledge(ctx)
		if role == "listingkit_viewer" {
			require.ErrorIs(t, err, k.ErrForbidden)
			require.Empty(t, scope)
		} else {
			require.NoError(t, err)
			require.Equal(t, k.Scope{OrganizationID: "org", ActorID: "actor"}, scope)
		}
	}
	require.Equal(t, 4, g.calls)
	g.revoked = true
	_, err = a.AuthorizeKnowledge(ctx)
	require.ErrorIs(t, err, k.ErrForbidden)
	g.revoked = false
	g.err = errors.New("identity provider unavailable")
	_, err = a.AuthorizeKnowledge(ctx)
	require.ErrorIs(t, err, k.ErrForbidden)
	_, err = a.AuthorizeKnowledge(context.Background())
	require.ErrorIs(t, err, k.ErrForbidden)
	request.Identity.TokenExpiresAt = time.Now().Add(-time.Second)
	_, err = a.AuthorizeKnowledge(commercetoolauth.WithOrganizationRequest(context.Background(), request))
	require.ErrorIs(t, err, k.ErrForbidden)
}
