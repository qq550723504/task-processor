package supplychainapp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/listing/preparation"
	"task-processor/internal/product/collection"
	"task-processor/internal/storecenter"
)

type organizationStatus struct{ suspended atomic.Bool }

func (s *organizationStatus) IsOrganizationSuspended(context.Context, string) (bool, error) {
	return s.suspended.Load(), nil
}

type currentRolePolicy struct{ removed atomic.Bool }

func (p *currentRolePolicy) RoleModules(_ context.Context, org string, keys []string) (map[string][]string, error) {
	values := map[string][]string{}
	if !p.removed.Load() {
		values[authz.EnterpriseRoleKey(org, 1)] = []string{"data-mine"}
	}
	return values, nil
}
func TestSupplyExecutionChecksOriginalGrantCurrentRoleAndOrganizationStatus(t *testing.T) {
	var replaced atomic.Bool
	var role atomic.Value
	role.Store("listingkit_admin")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		member := "member-a"
		if replaced.Load() {
			member = "replacement-member"
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"pagination":{"totalResult":"1"},"authorizations":[{"id":%q,"project":{"id":"project-a"},"organization":{"id":"org-a"},"user":{"id":"actor-a"},"state":"STATE_ACTIVE","roles":[{"key":%q}]}]}`, member, role.Load().(string))
	}))
	defer server.Close()
	permissions, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	policy := &currentRolePolicy{}
	permissions.SetRolePolicyReader(policy)
	status := &organizationStatus{}
	owner := OrganizationExecutionAuthorizer{Client: zitadel.NewAuthorizationClient(server.URL, server.Client()), ServiceToken: func(context.Context) (string, error) { return "isolated-service-fixture", nil }, ProjectID: "project-a", Permissions: permissions, OrganizationStatus: status}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	scope := collection.Scope{"org-a", "actor-a", "member-a"}
	require.NoError(t, owner.AuthorizeExecution(ctx, scope, collection.PermissionRead))
	roles, err := owner.ResolveAgentExecution(ctx, scope)
	require.NoError(t, err)
	require.Contains(t, roles, "listingkit_admin")
	owner.OrganizationStatus = nil
	require.NoError(t, owner.AuthorizeExecution(ctx, scope, collection.PermissionRead), "an unconfigured local overlay must match the current Workbench contract")
	owner.OrganizationStatus = status
	_, authenticated := authidentity.AuthenticatedIdentityFromContext(ctx)
	require.False(t, authenticated)
	replaced.Store(true)
	_, err = owner.ResolveAgentExecution(ctx, scope)
	require.ErrorIs(t, err, collection.ErrForbidden)
	require.ErrorIs(t, owner.AuthorizeExecution(ctx, scope, collection.PermissionRead), collection.ErrForbidden)
	replaced.Store(false)
	status.suspended.Store(true)
	require.ErrorIs(t, owner.AuthorizeExecution(ctx, scope, preparation.PermissionSubmit), collection.ErrForbidden)
	status.suspended.Store(false)
	role.Store(authz.EnterpriseRoleKey(scope.OrganizationID, 1))
	require.NoError(t, owner.AuthorizeExecution(ctx, scope, collection.PermissionRead))
	policy.removed.Store(true)
	require.ErrorIs(t, owner.AuthorizeExecution(ctx, scope, collection.PermissionRead), collection.ErrForbidden)
	policy.removed.Store(false)
	_, err = owner.AuthorizeProductExecution(ctx, storecenter.ProductExecutionSubject{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, MemberID: scope.MemberID, Purpose: storecenter.ProductPurposePublish})
	require.Error(t, err, "data access does not grant store publishing")
	role.Store("listingkit_admin")
	grant, err := owner.AuthorizeProductExecution(ctx, storecenter.ProductExecutionSubject{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, MemberID: scope.MemberID, Purpose: storecenter.ProductPurposePublish})
	require.NoError(t, err)
	require.True(t, grant.Allowed)
	require.Equal(t, scope.MemberID, grant.Access.MemberID)
	require.True(t, grant.Access.Administrator)
}
