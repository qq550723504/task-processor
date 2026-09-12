package membership

import (
	"context"
	"slices"
	"strings"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
)

type Service struct {
	directory      Directory
	authorizer     Authorizer
	projectID      string
	protectedRoles map[string]bool
}

func (s *Service) Read(ctx context.Context, id string) (Result, error) {
	identity, err := s.authorize(ctx, authz.PermissionWorkbenchOrganizationMemberRead)
	if err != nil {
		return Result{}, err
	}
	if !authidentity.IsBoundedIdentifier(id) {
		return Result{}, ErrInvalidRequest
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	member, err := s.directory.Read(ctx, identity.EffectiveOrganizationID, id)
	if ctx.Err() != nil {
		return Result{}, ErrUnavailable
	}
	if !time.Now().Before(identity.TokenExpiresAt) {
		return Result{}, ErrAuthentication
	}
	if err == ErrNotFound {
		return Result{}, ErrNotFound
	}
	if err != nil {
		return Result{}, ErrUnavailable
	}
	if member.ID != id || !authidentity.IsBoundedIdentifier(member.UserID) || member.OrganizationID != identity.EffectiveOrganizationID || member.ProjectID != s.projectID {
		return Result{}, ErrInvalidResponse
	}
	member.Roles = append([]string{}, member.Roles...)
	return s.project(identity, []Member{member}, 1), nil
}

func NewService(directory Directory, authorizer Authorizer, projectID string, protectedRoles ...string) *Service {
	protected := make(map[string]bool, len(protectedRoles))
	for _, role := range protectedRoles {
		protected[strings.TrimSpace(role)] = true
	}
	return &Service{directory: directory, authorizer: authorizer, projectID: projectID, protectedRoles: protected}
}

func (s *Service) List(ctx context.Context, page PageRequest) (Result, error) {
	identity, err := s.authorize(ctx, authz.PermissionWorkbenchOrganizationMemberRead)
	if err != nil {
		return Result{}, err
	}
	if page.Limit < 1 || page.Limit > 100 || page.Offset < 0 || page.Offset > 10000 {
		return Result{}, ErrInvalidRequest
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	listed, err := s.directory.List(ctx, identity.EffectiveOrganizationID, page)
	if err != nil || ctx.Err() != nil {
		return Result{}, ErrUnavailable
	}
	if !time.Now().Before(identity.TokenExpiresAt) {
		return Result{}, ErrAuthentication
	}
	if listed.Total < 0 || listed.Total > 10000 || len(listed.Items) > page.Limit || len(listed.Items) > listed.Total {
		return Result{}, ErrInvalidResponse
	}
	seen := make(map[string]bool)
	items := make([]Member, 0, len(listed.Items))
	for _, member := range listed.Items {
		if !authidentity.IsBoundedIdentifier(member.ID) || !authidentity.IsBoundedIdentifier(member.UserID) || member.OrganizationID != identity.EffectiveOrganizationID || member.ProjectID != s.projectID || seen[member.ID] {
			return Result{}, ErrInvalidResponse
		}
		seen[member.ID] = true
		member.Roles = append([]string{}, member.Roles...)
		items = append(items, member)
	}
	return s.project(identity, items, listed.Total), nil
}

// authorize consumes the verified live-grant identity installed by the current
// HTTP boundary. A matching grant is a prerequisite, not a read/manage grant.
func (s *Service) authorize(ctx context.Context, permission string) (authidentity.AuthenticatedIdentity, error) {
	if s == nil || s.directory == nil || s.authorizer == nil || !authidentity.IsBoundedIdentifier(s.projectID) {
		return authidentity.AuthenticatedIdentity{}, ErrUnavailable
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || !authidentity.IsBoundedIdentifier(identity.UserID) || !time.Now().Before(identity.TokenExpiresAt) {
		return authidentity.AuthenticatedIdentity{}, ErrAuthentication
	}
	if !authidentity.IsBoundedIdentifier(identity.EffectiveOrganizationID) {
		return authidentity.AuthenticatedIdentity{}, ErrPermission
	}
	matched := false
	for _, grant := range identity.OrganizationGrants {
		if grant.OrganizationID == identity.EffectiveOrganizationID && grant.ProjectID == s.projectID {
			if matched || !sameRoles(grant.Roles, identity.Roles) {
				return authidentity.AuthenticatedIdentity{}, ErrPermission
			}
			matched = true
		}
	}
	if !matched || !s.authorizer.Authorize(identity.UserID, identity.Roles, permission) {
		return authidentity.AuthenticatedIdentity{}, ErrPermission
	}
	if ctx.Err() != nil {
		return authidentity.AuthenticatedIdentity{}, ErrUnavailable
	}
	return identity, nil
}

func sameRoles(left, right []string) bool {
	left = append([]string{}, left...)
	right = append([]string{}, right...)
	slices.Sort(left)
	slices.Sort(right)
	return slices.Equal(left, right)
}
