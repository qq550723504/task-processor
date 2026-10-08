package httpapi

import (
	"context"
	"task-processor/internal/authz"
)

type currentRolePolicyFixture struct{}

func (currentRolePolicyFixture) RoleModules(_ context.Context, org string, keys []string) (map[string][]string, error) {
	result := map[string][]string{}
	for _, key := range keys {
		if key == authz.EnterpriseRoleKey(org, 1) {
			result[key] = []string{"members", "plans", "tasks", "source-accounts"}
		}
		if key == authz.EnterpriseRoleKey(org, 2) {
			result[key] = []string{"source-accounts", "knowledge", "stores", "acquisition", "agents", "images"}
		}
	}
	return result, nil
}
func newCurrentRoleTestAuthorizer(users, roles []string) (*authz.ListingKitAuthorizer, error) {
	a, err := authz.NewListingKitAuthorizer(users, roles)
	if err == nil {
		a.SetRolePolicyReader(currentRolePolicyFixture{})
	}
	return a, err
}
func currentRoleTestAuthorizer() *authz.ListingKitAuthorizer {
	a, _ := newCurrentRoleTestAuthorizer(nil, nil)
	return a
}
