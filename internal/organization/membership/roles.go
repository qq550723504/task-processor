package membership

import (
	"context"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
)

type RoleDefinition struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Modules []string `json:"modules"`
	Version int64    `json:"version"`
	System  bool     `json:"system"`
}
type RoleMutation struct {
	RoleID          string   `json:"-"`
	Name            string   `json:"name,omitempty"`
	Modules         []string `json:"modules"`
	ExpectedVersion int64    `json:"expectedVersion,omitempty"`
}
type RoleStore interface {
	Roles(context.Context, string) ([]RoleDefinition, int, error)
	MutateRole(context.Context, OperationScope, string, RoleMutation) (RoleDefinition, error)
}
type RolesResult struct {
	SchemaVersion  string             `json:"schemaVersion"`
	UserID         string             `json:"userId"`
	OrganizationID string             `json:"organizationId"`
	Items          []RoleDefinition   `json:"items"`
	Catalog        []authz.MenuModule `json:"catalog"`
	CanManage      bool               `json:"canManage"`
	CanCreate      bool               `json:"canCreate"`
	RemainingSlots int                `json:"remainingSlots"`
}
type RoleMutationResult struct {
	SchemaVersion  string         `json:"schemaVersion"`
	UserID         string         `json:"userId"`
	OrganizationID string         `json:"organizationId"`
	OperationID    string         `json:"operationId"`
	Role           RoleDefinition `json:"role"`
}

func (input RoleMutation) Normalize(organization string) (RoleMutation, error) {
	input.Modules = append([]string{}, input.Modules...)
	if !authz.ValidModuleIDs(input.Modules) {
		return RoleMutation{}, ErrInvalidRequest
	}
	slices.Sort(input.Modules)
	if input.RoleID == "" {
		input.Name = strings.TrimSpace(input.Name)
		if !utf8.ValidString(input.Name) || utf8.RuneCountInString(input.Name) < 1 || utf8.RuneCountInString(input.Name) > 40 || strings.ContainsFunc(input.Name, unicode.IsControl) || input.ExpectedVersion != 0 {
			return RoleMutation{}, ErrInvalidRequest
		}
	} else if !authz.IsEnterpriseRoleKey(organization, input.RoleID) || input.Name != "" || input.ExpectedVersion < 1 || input.ExpectedVersion > 1000000000 {
		return RoleMutation{}, ErrInvalidRequest
	}
	return input, nil
}

func (s *Service) SetRoleStore(store RoleStore) { s.roleStore = store }

func (s *Service) Roles(ctx context.Context) (RolesResult, error) {
	identity, err := s.authorize(ctx, authz.PermissionWorkbenchOrganizationMemberRead)
	if err != nil {
		return RolesResult{}, err
	}
	if s.roleStore == nil {
		return RolesResult{}, ErrUnavailable
	}
	items, remaining, err := s.roleStore.Roles(ctx, identity.EffectiveOrganizationID)
	if err != nil {
		return RolesResult{}, err
	}
	modules := []string{}
	for _, m := range authz.MenuModules() {
		if m.Available {
			modules = append(modules, m.ID)
		}
	}
	items = append([]RoleDefinition{{ID: "listingkit_admin", Name: "管理员", Modules: modules, Version: 0, System: true}}, items...)
	manage := authz.AllowedOrganization(ctx, s.authorizer, identity.UserID, identity.EffectiveOrganizationID, identity.Roles, authz.PermissionWorkbenchOrganizationMemberManage)
	return RolesResult{SchemaVersion: "enterprise-roles-v1", UserID: identity.UserID, OrganizationID: identity.EffectiveOrganizationID, Items: items, Catalog: authz.MenuModules(), CanManage: manage, CanCreate: manage && remaining > 0, RemainingSlots: remaining}, nil
}

func (c *Commands) MutateRole(ctx context.Context, key string, input RoleMutation) (RoleMutationResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ctx, identity, err := c.current(ctx)
	if err != nil {
		return RoleMutationResult{}, err
	}
	id, err := uuid.Parse(key)
	if err != nil || id == uuid.Nil || id.String() != key {
		return RoleMutationResult{}, ErrInvalidRequest
	}
	input, err = input.Normalize(identity.EffectiveOrganizationID)
	if err != nil {
		return RoleMutationResult{}, err
	}
	if c.service.roleStore == nil {
		return RoleMutationResult{}, ErrUnavailable
	}
	if _, err = c.service.authorize(ctx, authz.PermissionWorkbenchOrganizationMemberManage); err != nil {
		return RoleMutationResult{}, err
	}
	if ctx.Err() != nil || !time.Now().Before(identity.TokenExpiresAt) {
		return RoleMutationResult{}, ErrAuthentication
	}
	ctx, expiryCancel := context.WithDeadline(ctx, identity.TokenExpiresAt)
	defer expiryCancel()
	role, err := c.service.roleStore.MutateRole(ctx, OperationScope{ProjectID: c.service.projectID, OrganizationID: identity.EffectiveOrganizationID, ActorID: identity.UserID}, key, input)
	if err != nil {
		return RoleMutationResult{}, err
	}
	return RoleMutationResult{SchemaVersion: "enterprise-role-mutation-v1", UserID: identity.UserID, OrganizationID: identity.EffectiveOrganizationID, OperationID: key, Role: role}, nil
}

func validRoleScope(scope OperationScope) bool {
	return authidentity.IsBoundedIdentifier(scope.ProjectID) && authidentity.IsBoundedIdentifier(scope.OrganizationID) && authidentity.IsBoundedIdentifier(scope.ActorID)
}
