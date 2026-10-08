package membership

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
)

func (s *Service) assignable(ctx context.Context, organization, role string) bool {
	if s.protectedRoles[role] {
		return false
	}
	if role == "listingkit_admin" {
		return true
	}
	if s.roleStore == nil || !authz.IsEnterpriseRoleKey(organization, role) {
		return false
	}
	roles, _, err := s.roleStore.Roles(ctx, organization)
	if err != nil {
		return false
	}
	for _, r := range roles {
		if r.ID == role && !r.System {
			return true
		}
	}
	return false
}
func (s *Service) editable(ctx context.Context, organization string, member Member) bool {
	if len(member.Roles) == 0 || s.authorizer.Authorize(member.UserID, member.Roles, authz.PermissionListingKitPlatformAdm) {
		return false
	}
	for _, role := range member.Roles {
		if !s.assignable(ctx, organization, role) {
			return false
		}
	}
	return true
}

func (s *Service) project(ctx context.Context, identity authidentity.AuthenticatedIdentity, items []Member, total int) (Result, error) {
	manage := authz.AllowedOrganization(ctx, s.authorizer, identity.UserID, identity.EffectiveOrganizationID, identity.Roles, authz.PermissionWorkbenchOrganizationMemberManage)
	definitions := []RoleDefinition{{ID: "listingkit_admin", Name: "管理员", Modules: []string{}, System: true}}
	if s.roleStore != nil {
		custom, _, err := s.roleStore.Roles(ctx, identity.EffectiveOrganizationID)
		if err != nil {
			return Result{}, ErrUnavailable
		}
		definitions = append(definitions, custom...)
	}
	roles := []string{}
	if manage {
		for _, role := range definitions {
			if !s.protectedRoles[role.ID] {
				roles = append(roles, role.ID)
			}
		}
	}
	for i := range items {
		items[i].Permissions = []string{}
		if items[i].State == "active" {
			permissions, err := authz.PermissionsInOrganization(ctx, s.authorizer, items[i].UserID, identity.EffectiveOrganizationID, items[i].Roles)
			if err != nil {
				return Result{}, ErrUnavailable
			}
			items[i].Permissions = permissions
		}
		items[i].ObservedVersion = observedVersion(items[i])
		items[i].CanChangeRole = manage && len(roles) > 0 && s.editable(ctx, identity.EffectiveOrganizationID, items[i])
		items[i].CanRemove = manage && s.editable(ctx, identity.EffectiveOrganizationID, items[i])
	}
	return Result{SchemaVersion: "membership-v1", UserID: identity.UserID, OrganizationID: identity.EffectiveOrganizationID, Items: items, Total: total, CanManage: manage, AssignableRoles: roles, RoleDefinitions: definitions}, nil
}

// observedVersion describes provider authorization facts. It is never provider CAS.
func observedVersion(member Member) string {
	roles := append([]string{}, member.Roles...)
	slices.Sort(roles)
	data, _ := json.Marshal([]any{member.ID, member.UserID, member.OrganizationID, member.ProjectID, roles, member.State, member.ChangedAt})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
