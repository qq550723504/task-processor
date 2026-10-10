package supplymarket

import (
	"context"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/sourcing"
	"time"
)

type ContextAuthorizer struct {
	live        sourcing.LiveOrganizationAccess
	permissions *authz.ListingKitAuthorizer
}

func NewContextAuthorizer(live sourcing.LiveOrganizationAccess, permissions *authz.ListingKitAuthorizer) (*ContextAuthorizer, error) {
	if live == nil || permissions == nil {
		return nil, ErrUnavailable
	}
	return &ContextAuthorizer{live, permissions}, nil
}
func (a *ContextAuthorizer) Authorize(ctx context.Context, permission string) (collection.Scope, error) {
	if ctx == nil || ctx.Err() != nil || a == nil || a.live == nil || a.permissions == nil {
		return collection.Scope{}, ErrForbidden
	}
	if permission != PermissionRead && permission != PermissionSelect && permission != PermissionApply && permission != PermissionDesign && permission != collection.PermissionManage && permission != collection.PermissionRead {
		return collection.Scope{}, ErrForbidden
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	scope := collection.Scope{OrganizationID: identity.EffectiveOrganizationID, ActorID: identity.UserID, MemberID: identity.EffectiveMemberID}
	if !ok || scope.Validate() != nil || identity.TenantID != scope.OrganizationID || !time.Now().Before(identity.TokenExpiresAt) {
		return collection.Scope{}, ErrForbidden
	}
	roles, err := a.live.ResolveLiveRoles(ctx, scope.OrganizationID, scope.ActorID)
	if err != nil {
		return collection.Scope{}, ErrForbidden
	}
	allowed, err := authz.AuthorizeOrganization(ctx, a.permissions, scope.ActorID, scope.OrganizationID, roles, permission)
	if err != nil {
		return collection.Scope{}, ErrUnavailable
	}
	if !allowed || ctx.Err() != nil || !time.Now().Before(identity.TokenExpiresAt) {
		return collection.Scope{}, ErrForbidden
	}
	return scope, nil
}
func (a *ContextAuthorizer) AuthorizePlatform(ctx context.Context) (string, error) {
	if ctx == nil || ctx.Err() != nil || a == nil || a.permissions == nil {
		return "", ErrForbidden
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || !authidentity.IsBoundedIdentifier(identity.UserID) || identity.TenantID != "" || identity.EffectiveOrganizationID != "" || identity.EffectiveMemberID != "" || !time.Now().Before(identity.TokenExpiresAt) || !a.permissions.Authorize(identity.UserID, identity.Roles, authz.PermissionListingKitPlatformAdm) {
		return "", ErrForbidden
	}
	return identity.UserID, nil
}
