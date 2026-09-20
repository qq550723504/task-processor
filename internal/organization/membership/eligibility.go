package membership

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
)

var ordinaryRoles = []string{"listingkit_viewer", "listingkit_operator", "listingkit_admin"}

func (s *Service) assignable(role string) bool {
	return slices.Contains(ordinaryRoles, role) && !s.protectedRoles[role]
}
func (s *Service) editable(member Member) bool {
	if len(member.Roles) == 0 {
		return false
	}
	for _, role := range member.Roles {
		if !s.assignable(role) {
			return false
		}
	}
	return true
}

func (s *Service) project(identity authidentity.AuthenticatedIdentity, items []Member, total int) Result {
	manage := s.authorizer.Authorize(identity.UserID, identity.Roles, authz.PermissionWorkbenchOrganizationMemberManage)
	roles := []string{}
	if manage {
		for _, role := range ordinaryRoles {
			if s.assignable(role) {
				roles = append(roles, role)
			}
		}
	}
	for i := range items {
		items[i].ObservedVersion = observedVersion(items[i])
		items[i].CanChangeRole = manage && len(roles) > 0 && s.editable(items[i])
		items[i].CanRemove = manage && s.editable(items[i])
	}
	return Result{SchemaVersion: "membership-v1", UserID: identity.UserID, OrganizationID: identity.EffectiveOrganizationID, Items: items, Total: total, CanManage: manage, AssignableRoles: roles}
}

// observedVersion describes provider authorization facts. It is never provider CAS.
func observedVersion(member Member) string {
	roles := append([]string{}, member.Roles...)
	slices.Sort(roles)
	data, _ := json.Marshal([]any{member.ID, member.UserID, member.OrganizationID, member.ProjectID, roles, member.State, member.ChangedAt})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
