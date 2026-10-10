package httpapi

import (
	"context"
	"github.com/stretchr/testify/require"
	pc "task-processor/internal/aiworkbench/projectcenter"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/knowledge"
	"task-processor/internal/storecenter"
	"testing"
	"time"
)

type projectOnlyPolicy struct{ key string }

func (p projectOnlyPolicy) RoleModules(_ context.Context, org string, keys []string) (map[string][]string, error) {
	return map[string][]string{p.key: {"projects"}}, nil
}
func TestProjectCenterGrantDoesNotAuthorizeOriginalOwners(t *testing.T) {
	key := authz.EnterpriseRoleKey("org-a", 1)
	policy, e := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, e)
	policy.SetRolePolicyReader(projectOnlyPolicy{key})
	id := authidentity.AuthenticatedIdentity{UserID: "actor-a", TenantID: "org-a", EffectiveOrganizationID: "org-a", EffectiveMemberID: "member-a", Roles: []string{key}, TokenExpiresAt: time.Now().Add(time.Hour)}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), id)
	ctx, e = (productReviewCapabilityBinder{}).Bind(ctx, "Bearer fixture-only")
	require.NoError(t, e)
	source := &projectSources{resolver: &productReviewResolverStub{value: id}, policy: policy, chat: &aiWorkbenchApplication{}, knowledge: &knowledge.Service{}, stores: &storecenter.MemberScopedStoreRepository{}}
	scope := pc.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	require.NoError(t, source.authorize(ctx, scope, true))
	// Non-nil but uninitialized source owners would panic if their original grants were bypassed.
	for _, kind := range []string{"CONVERSATION", "BUSINESS_TASK", "KNOWLEDGE_BASE", "KNOWLEDGE_SOURCE", "STORE", "PRODUCT"} {
		v, e := source.Resolve(ctx, scope, pc.Reference{Kind: kind, TargetID: "550e8400-e29b-41d4-a716-446655440000"})
		require.Error(t, e, kind)
		require.Empty(t, v.TargetID)
		require.Empty(t, v.Title)
		require.Empty(t, v.ResultHref)
	}
	source.resolver = &productReviewResolverStub{value: authidentity.AuthenticatedIdentity{UserID: id.UserID, TenantID: id.TenantID, EffectiveOrganizationID: id.EffectiveOrganizationID, EffectiveMemberID: "new-member", TokenExpiresAt: id.TokenExpiresAt, Roles: id.Roles}}
	require.ErrorIs(t, source.authorize(ctx, scope, true), pc.ErrForbidden)
}
