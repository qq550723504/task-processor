package preparation

import (
	"context"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/product/sourcing"
	"time"
)

type ContextAuthorizer struct {
	live        sourcing.LiveOrganizationAccess
	permissions *authz.ListingKitAuthorizer
	now         func() time.Time
}

func NewContextAuthorizer(live sourcing.LiveOrganizationAccess, permissions *authz.ListingKitAuthorizer) (*ContextAuthorizer, error) {
	if live == nil || permissions == nil {
		return nil, ErrUnavailable
	}
	return &ContextAuthorizer{live, permissions, time.Now}, nil
}
func (a *ContextAuthorizer) Authorize(ctx context.Context, purpose string) (Scope, error) {
	if ctx == nil || ctx.Err() != nil || a == nil || a.live == nil || a.permissions == nil {
		return Scope{}, ErrForbidden
	}
	if purpose != PermissionRead && purpose != PermissionManage && purpose != PermissionSubmit {
		return Scope{}, ErrForbidden
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	scope := Scope{OrganizationID: identity.EffectiveOrganizationID, ActorID: identity.UserID, MemberID: identity.EffectiveMemberID}
	if !ok || scope.Validate() != nil || identity.TenantID != scope.OrganizationID || !a.now().Before(identity.TokenExpiresAt) {
		return Scope{}, ErrForbidden
	}
	roles, err := a.live.ResolveLiveRoles(ctx, scope.OrganizationID, scope.ActorID)
	if err != nil {
		return Scope{}, ErrForbidden
	}
	allowed, err := authz.AuthorizeOrganization(ctx, a.permissions, scope.ActorID, scope.OrganizationID, roles, purpose)
	if err != nil {
		return Scope{}, ErrUnavailable
	}
	if !allowed || ctx.Err() != nil || !a.now().Before(identity.TokenExpiresAt) {
		return Scope{}, ErrForbidden
	}
	return scope, nil
}
