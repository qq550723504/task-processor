package imageagentworker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/imageagent"
)

func TestOrganizationExecutionAuthorizerRejectsReplacedMemberGrant(t *testing.T) {
	var replaced atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if replaced.Load() {
			_, _ = w.Write([]byte(`{"pagination":{"totalResult":"1"},"authorizations":[{"id":"new-member","project":{"id":"project-1"},"organization":{"id":"org-1"},"user":{"id":"actor-1"},"state":"STATE_ACTIVE","roles":[{"key":"listingkit_admin"}]}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"pagination":{"totalResult":"1"},"authorizations":[{"id":"old-member","project":{"id":"project-1"},"organization":{"id":"org-1"},"user":{"id":"actor-1"},"state":"STATE_ACTIVE","roles":[{"key":"listingkit_admin"}]}]}`))
	}))
	defer server.Close()
	authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	auth := OrganizationExecutionAuthorizer{
		Client:       zitadel.NewAuthorizationClient(server.URL, server.Client()),
		ServiceToken: func(context.Context) (string, error) { return "service-token", nil },
		ProjectID:    "project-1", Authorizer: authorizer,
	}
	identity := imageagent.ExecutionIdentity{
		ScopeProtocol: imageagent.OrganizationScopeProtocol,
		RunID:         "run-1", TenantID: "org-1", UserID: "actor-1", MemberID: "old-member", BusinessTaskID: "operation-1",
	}
	require.NoError(t, auth.AuthorizeExecution(context.Background(), identity))
	replaced.Store(true)
	require.ErrorIs(t, auth.AuthorizeExecution(context.Background(), identity), imageagent.ErrIdentityRequired)
	identity.MemberID = "new-member"
	require.NoError(t, auth.AuthorizeExecution(context.Background(), identity))
}

func TestOrganizationExecutionAuthorizerRejectsMultipleActiveGrantsWithoutRoleUnion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"pagination":{"totalResult":"2"},"authorizations":[{"id":"member-viewer","project":{"id":"project-1"},"organization":{"id":"org-1"},"user":{"id":"actor-1"},"state":"STATE_ACTIVE","roles":[{"key":"listingkit_viewer"}]},{"id":"member-admin","project":{"id":"project-1"},"organization":{"id":"org-1"},"user":{"id":"actor-1"},"state":"STATE_ACTIVE","roles":[{"key":"listingkit_admin"}]}]}`))
	}))
	defer server.Close()
	authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	auth := OrganizationExecutionAuthorizer{
		Client:       zitadel.NewAuthorizationClient(server.URL, server.Client()),
		ServiceToken: func(context.Context) (string, error) { return "service-token", nil },
		ProjectID:    "project-1", Authorizer: authorizer,
	}
	identity := imageagent.ExecutionIdentity{
		ScopeProtocol: imageagent.OrganizationScopeProtocol,
		RunID:         "run-1", TenantID: "org-1", UserID: "actor-1", MemberID: "member-viewer", BusinessTaskID: "operation-1",
	}
	require.ErrorIs(t, auth.AuthorizeExecution(context.Background(), identity), imageagent.ErrIdentityRequired)
	identity.MemberID = "member-admin"
	require.ErrorIs(t, auth.AuthorizeExecution(context.Background(), identity), imageagent.ErrIdentityRequired)
}
