package authz

import (
	"strings"
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

const (
	PermissionListingKitAdminRead   = "listingkit.admin.read"
	PermissionListingKitAdminWrite  = "listingkit.admin.write"
	PermissionListingKitPlatformAdm = "listingkit.platform_admin"
	PermissionProductSourcingWrite  = "product_sourcing.write"
)

const listingKitModel = `
[request_definition]
r = sub, obj

[policy_definition]
p = sub, obj

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = (r.sub == p.sub || g(r.sub, p.sub)) && r.obj == p.obj
`

type ListingKitAuthorizer struct {
	enforcer *casbin.Enforcer
}

var (
	defaultListingKitAuthorizerOnce sync.Once
	defaultListingKitAuthorizer     *ListingKitAuthorizer
)

func NewListingKitAuthorizer(platformAdminUsers []string, platformAdminRoles []string) (*ListingKitAuthorizer, error) {
	m, err := model.NewModelFromString(listingKitModel)
	if err != nil {
		return nil, err
	}
	enforcer, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil, err
	}

	for _, policy := range [][]string{
		{"listingkit_operator", PermissionListingKitAdminRead},
		{"listingkit_operator", PermissionListingKitAdminWrite},
		{"listingkit_operator", PermissionProductSourcingWrite},
		{"listingkit_admin", PermissionListingKitAdminRead},
		{"listingkit_admin", PermissionListingKitAdminWrite},
		{"listingkit_admin", PermissionListingKitPlatformAdm},
		{"listingkit_admin", PermissionProductSourcingWrite},
		{"platform_admin", PermissionListingKitAdminRead},
		{"platform_admin", PermissionListingKitAdminWrite},
		{"platform_admin", PermissionListingKitPlatformAdm},
		{"platform_admin", PermissionProductSourcingWrite},
		{"admin", PermissionListingKitPlatformAdm},
	} {
		if _, err := enforcer.AddPolicy(policy); err != nil {
			return nil, err
		}
	}

	for _, role := range normalizeUnique(platformAdminRoles) {
		if _, err := enforcer.AddPolicy(role, PermissionListingKitPlatformAdm); err != nil {
			return nil, err
		}
	}
	for _, userID := range normalizeUnique(platformAdminUsers) {
		subject := userSubject(userID)
		if _, err := enforcer.AddPolicy(subject, PermissionListingKitPlatformAdm); err != nil {
			return nil, err
		}
	}

	return &ListingKitAuthorizer{enforcer: enforcer}, nil
}

func (a *ListingKitAuthorizer) Authorize(userID string, roles []string, permission string) bool {
	if a == nil || a.enforcer == nil {
		return false
	}
	permission = strings.TrimSpace(permission)
	if permission == "" {
		return true
	}

	if subject := userSubject(userID); subject != "" {
		ok, err := a.enforcer.Enforce(subject, permission)
		if err == nil && ok {
			return true
		}
	}

	for _, role := range normalizeUnique(roles) {
		ok, err := a.enforcer.Enforce(role, permission)
		if err == nil && ok {
			return true
		}
	}
	return false
}

func DefaultListingKitAuthorizer() *ListingKitAuthorizer {
	defaultListingKitAuthorizerOnce.Do(func() {
		defaultListingKitAuthorizer, _ = NewListingKitAuthorizer(nil, nil)
	})
	return defaultListingKitAuthorizer
}

func IsListingKitPlatformAdmin(userID string, roles []string) bool {
	return DefaultListingKitAuthorizer().Authorize(userID, roles, PermissionListingKitPlatformAdm)
}

func userSubject(userID string) string {
	if value := strings.TrimSpace(userID); value != "" {
		return "user:" + value
	}
	return ""
}

func normalizeUnique(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, item := range values {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
