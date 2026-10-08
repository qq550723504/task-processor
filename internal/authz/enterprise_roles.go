package authz

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"slices"
	"strings"
	"task-processor/internal/authidentity"
)

const EnterpriseRoleCapacity = 64

type RolePolicyReader interface {
	RoleModules(context.Context, string, []string) (map[string][]string, error)
}

func EnterpriseRoleKey(organization string, slot int) string {
	digest := sha256.Sum256([]byte(organization))
	return fmt.Sprintf("sumi_role_%x_%02d", digest[:16], slot)
}

func (a *ListingKitAuthorizer) SetRolePolicyReader(reader RolePolicyReader) {
	a.rolePolicyReader = reader
}

func (a *ListingKitAuthorizer) AuthorizeScoped(ctx context.Context, userID, organization string, roles []string, permission string) (bool, error) {
	static, modules, err := a.scopedPolicy(ctx, organization, roles)
	if err != nil {
		return false, err
	}
	return a.scopedAllows(userID, static, modules, permission), nil
}

var ErrRolePolicyUnavailable = errors.New("enterprise role policy unavailable")

func IsEnterpriseRoleKey(organization, key string) bool {
	if !authidentity.IsBoundedIdentifier(organization) {
		return false
	}
	for slot := 1; slot <= EnterpriseRoleCapacity; slot++ {
		if key == EnterpriseRoleKey(organization, slot) {
			return true
		}
	}
	return false
}

func newModuleEnforcer() (*casbin.Enforcer, error) {
	m, err := model.NewModelFromString(listingKitModel)
	if err != nil {
		return nil, err
	}
	e, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil, err
	}
	for _, module := range enterpriseModules {
		for _, permission := range module.Permissions {
			if _, err = e.AddPolicy(module.ID, permission); err != nil {
				return nil, err
			}
		}
	}
	return e, nil
}

func (a *ListingKitAuthorizer) scopedPolicy(ctx context.Context, organization string, roles []string) ([]string, []string, error) {
	if a == nil || ctx == nil || ctx.Err() != nil || !authidentity.IsBoundedIdentifier(organization) || len(roles) > 64 {
		return nil, nil, ErrRolePolicyUnavailable
	}
	static, keys := []string{}, []string{}
	for _, role := range normalizeUnique(roles) {
		if a.protectedRoles[role] {
			static = append(static, role)
			continue
		}
		if IsEnterpriseRoleKey(organization, role) {
			keys = append(keys, role)
			continue
		}
		if role == "listingkit_viewer" || role == "listingkit_operator" {
			if !a.protectedRoles[role] {
				continue
			}
		}
		if !strings.HasPrefix(role, "sumi_role_") {
			static = append(static, role)
		}
	}
	modules := []string{}
	if len(keys) > 0 {
		if a.rolePolicyReader == nil {
			return nil, nil, ErrRolePolicyUnavailable
		}
		policies, err := a.rolePolicyReader.RoleModules(ctx, organization, keys)
		if err != nil {
			return nil, nil, ErrRolePolicyUnavailable
		}
		for key, ids := range policies {
			if !slices.Contains(keys, key) || !ValidModuleIDs(ids) {
				return nil, nil, ErrRolePolicyUnavailable
			}
			for _, id := range ids {
				if !slices.Contains(modules, id) {
					modules = append(modules, id)
				}
			}
		}
	}
	return static, modules, nil
}

func (a *ListingKitAuthorizer) scopedAllows(user string, roles, modules []string, permission string) bool {
	if a.Authorize(user, roles, permission) {
		return true
	}
	if a.moduleEnforcer == nil {
		return false
	}
	for _, module := range modules {
		ok, err := a.moduleEnforcer.Enforce(module, permission)
		if err == nil && ok {
			return true
		}
	}
	return false
}

func (a *ListingKitAuthorizer) ScopedPermissions(ctx context.Context, user, organization string, roles []string) ([]string, error) {
	static, modules, err := a.scopedPolicy(ctx, organization, roles)
	if err != nil {
		return nil, err
	}
	result := []string{}
	for _, p := range WorkbenchPermissions() {
		if a.scopedAllows(user, static, modules, p) {
			result = append(result, p)
		}
	}
	return result, nil
}

type StaticAuthorizer interface {
	Authorize(string, []string, string) bool
}
type ScopedAuthorizer interface {
	AuthorizeScoped(context.Context, string, string, []string, string) (bool, error)
}

// AuthorizeOrganization adapts narrow existing domain ports. Custom roles always
// require the scoped policy contract; a static-only implementation cannot grant them.
func AuthorizeOrganization(ctx context.Context, a StaticAuthorizer, user, organization string, roles []string, permission string) (bool, error) {
	if a == nil {
		return false, ErrRolePolicyUnavailable
	}
	if scoped, ok := a.(ScopedAuthorizer); ok {
		return scoped.AuthorizeScoped(ctx, user, organization, roles, permission)
	}
	for _, role := range roles {
		if strings.HasPrefix(role, "sumi_role_") {
			return false, ErrRolePolicyUnavailable
		}
	}
	return a.Authorize(user, roles, permission), nil
}

func AllowedOrganization(ctx context.Context, a StaticAuthorizer, user, organization string, roles []string, permission string) bool {
	allowed, err := AuthorizeOrganization(ctx, a, user, organization, roles, permission)
	return err == nil && allowed
}

func PermissionsInOrganization(ctx context.Context, a StaticAuthorizer, user, organization string, roles []string) ([]string, error) {
	if batch, ok := a.(interface {
		ScopedPermissions(context.Context, string, string, []string) ([]string, error)
	}); ok {
		return batch.ScopedPermissions(ctx, user, organization, roles)
	}
	result := []string{}
	for _, p := range WorkbenchPermissions() {
		allowed, err := AuthorizeOrganization(ctx, a, user, organization, roles, p)
		if err != nil {
			return nil, err
		}
		if allowed {
			result = append(result, p)
		}
	}
	return result, nil
}
