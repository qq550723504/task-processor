package membership

import (
	"context"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authz"
	"testing"
)

type roleStoreFixture struct{ roles []RoleDefinition }

func (f roleStoreFixture) Roles(context.Context, string) ([]RoleDefinition, int, error) {
	return f.roles, 64 - len(f.roles), nil
}
func (f roleStoreFixture) MutateRole(context.Context, OperationScope, string, RoleMutation) (RoleDefinition, error) {
	return RoleDefinition{}, ErrUnavailable
}
func (f roleStoreFixture) RoleModules(_ context.Context, org string, keys []string) (map[string][]string, error) {
	result := map[string][]string{}
	for _, r := range f.roles {
		for _, key := range keys {
			if key == r.ID && authz.IsEnterpriseRoleKey(org, key) {
				result[key] = r.Modules
			}
		}
	}
	return result, nil
}

func TestCustomMemberRoleAndProtectedSubjectEligibility(t *testing.T) {
	key := authz.EnterpriseRoleKey("effective-b", 1)
	roles := roleStoreFixture{[]RoleDefinition{{ID: key, Name: "商品运营", Modules: []string{"acquisition"}, Version: 1}}}
	a, err := authz.NewListingKitAuthorizer([]string{"protected-member"}, nil)
	require.NoError(t, err)
	a.SetRolePolicyReader(roles)
	d := &directoryStub{page: Page{Total: 2, Items: []Member{
		{ID: "grant-one", UserID: "ordinary", OrganizationID: "effective-b", ProjectID: "project", Roles: []string{key}, State: "active"},
		{ID: "grant-two", UserID: "protected-member", OrganizationID: "effective-b", ProjectID: "project", Roles: []string{"listingkit_admin"}, State: "active"},
	}}}
	s := NewService(d, a, "project")
	s.SetRoleStore(roles)
	result, err := s.List(scopedContext("listingkit_admin"), PageRequest{Limit: 20})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"listingkit_admin", key}, result.AssignableRoles)
	require.True(t, result.Items[0].CanChangeRole)
	require.True(t, result.Items[0].CanRemove)
	require.Contains(t, result.Items[0].Permissions, authz.PermissionProductSourcingWrite)
	require.False(t, result.Items[1].CanChangeRole)
	require.False(t, result.Items[1].CanRemove)
}

var testReadRole = authz.EnterpriseRoleKey("effective-b", 1)
var testOperateRole = authz.EnterpriseRoleKey("effective-b", 2)

func testService(directory Directory, a Authorizer, project string, protected ...string) *Service {
	roles := roleStoreFixture{[]RoleDefinition{{ID: testReadRole, Name: "查看成员", Modules: []string{"members"}, Version: 1}, {ID: testOperateRole, Name: "商品运营", Modules: []string{"members", "acquisition", "source-accounts", "stores"}, Version: 1}}}
	if policy, ok := a.(*authz.ListingKitAuthorizer); ok {
		policy.SetRolePolicyReader(roles)
	}
	s := NewService(directory, a, project, protected...)
	s.SetRoleStore(roles)
	return s
}
