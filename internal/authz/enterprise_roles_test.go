package authz

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type rolePolicyFixture map[string]map[string][]string

func (f rolePolicyFixture) RoleModules(_ context.Context, organization string, keys []string) (map[string][]string, error) {
	result := map[string][]string{}
	for _, key := range keys {
		if modules, ok := f[organization][key]; ok {
			result[key] = modules
		}
	}
	return result, nil
}

type failedRolePolicy struct{}

func (failedRolePolicy) RoleModules(context.Context, string, []string) (map[string][]string, error) {
	return nil, errors.New("policy unavailable")
}

func TestEnterpriseRolePolicyIsScopedCurrentAndCannotEscalate(t *testing.T) {
	ctx := context.Background()
	key := EnterpriseRoleKey("org-a", 1)
	policy := rolePolicyFixture{"org-a": {key: {"knowledge"}}}
	a, err := NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	a.SetRolePolicyReader(policy)
	allowed, err := a.AuthorizeScoped(ctx, "member", "org-a", []string{key}, PermissionWorkbenchKnowledgeRead)
	require.NoError(t, err)
	require.True(t, allowed, "a saved module must authorize the actual business permission")
	for _, permission := range []string{PermissionWorkbenchOrganizationMemberManage, PermissionListingKitPlatformAdm, PermissionListingKitAdminRead, PermissionWorkbenchCommercialPurchase, PermissionWorkbenchAgentConfigure} {
		allowed, err = a.AuthorizeScoped(ctx, "member", "org-a", []string{key}, permission)
		require.NoError(t, err)
		require.False(t, allowed, permission)
	}
	allowed, err = a.AuthorizeScoped(ctx, "member", "org-b", []string{key}, PermissionWorkbenchKnowledgeRead)
	require.NoError(t, err)
	require.False(t, allowed, "native key cannot cross enterprises")
	allowed, err = a.AuthorizeScoped(ctx, "member", "org-a", []string{EnterpriseRoleKey("org-a", 2)}, PermissionWorkbenchKnowledgeRead)
	require.NoError(t, err)
	require.False(t, allowed, "unused native slots cannot authorize")
	require.False(t, a.Authorize("member", []string{key}, PermissionWorkbenchKnowledgeRead))
	policy["org-a"][key] = []string{}
	allowed, err = a.AuthorizeScoped(ctx, "member", "org-a", []string{key}, PermissionWorkbenchKnowledgeRead)
	require.NoError(t, err)
	require.False(t, allowed, "removed permission must not survive a process cache")
	a.SetRolePolicyReader(failedRolePolicy{})
	allowed, err = a.AuthorizeScoped(ctx, "member", "org-a", []string{key}, PermissionWorkbenchKnowledgeRead)
	require.Error(t, err)
	require.False(t, allowed)
}

func TestEnterpriseRoleKeepsSystemAndProtectedPolicy(t *testing.T) {
	a, err := NewListingKitAuthorizer([]string{"protected-user"}, []string{"protected-role"})
	require.NoError(t, err)
	for _, roles := range [][]string{{"listingkit_admin"}, {"protected-role"}} {
		allowed, err := a.AuthorizeScoped(context.Background(), "member", "org-a", roles, PermissionWorkbenchOrganizationMemberManage)
		require.NoError(t, err)
		require.True(t, allowed)
	}
	allowed, err := a.AuthorizeScoped(context.Background(), "protected-user", "org-a", nil, PermissionWorkbenchOrganizationMemberManage)
	require.NoError(t, err)
	require.True(t, allowed)
	for _, role := range []string{"listingkit_viewer", "listingkit_operator"} {
		allowed, err = a.AuthorizeScoped(context.Background(), "member", "org-a", []string{role}, PermissionWorkbenchChatRead)
		require.NoError(t, err)
		require.False(t, allowed, "retired role options are not custom role policies")
	}
}
