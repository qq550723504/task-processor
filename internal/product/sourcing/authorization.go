package sourcing

import (
	"context"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
)

// LiveOrganizationAccess is the existing identity/Organization boundary seen
// by an internal caller. It must read current grants rather than a request cache.
type LiveOrganizationAccess interface {
	ResolveLiveRoles(context.Context, string, string) ([]string, error)
}

// ContextAuthorizer derives Organization and actor only from verified context,
// then refreshes roles before evaluating product_sourcing.write.
type ContextAuthorizer struct {
	live        LiveOrganizationAccess
	permissions *authz.ListingKitAuthorizer
	now         func() time.Time
}

func NewContextAuthorizer(live LiveOrganizationAccess, permissions *authz.ListingKitAuthorizer) (*ContextAuthorizer, error) {
	if live == nil || permissions == nil {
		return nil, ErrSourcePublicationUnavailable
	}
	return &ContextAuthorizer{live: live, permissions: permissions, now: time.Now}, nil
}

func (a *ContextAuthorizer) Authorize(ctx context.Context) (PublicationScope, error) {
	if err := ctx.Err(); err != nil {
		return PublicationScope{}, err
	}
	if a == nil || a.live == nil || a.permissions == nil {
		return PublicationScope{}, ErrSourcePublicationUnavailable
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || !authidentity.IsBoundedIdentifier(identity.UserID) ||
		!authidentity.IsBoundedIdentifier(identity.EffectiveOrganizationID) ||
		identity.TenantID != identity.EffectiveOrganizationID || identity.TokenExpiresAt.IsZero() ||
		!a.now().Before(identity.TokenExpiresAt) {
		return PublicationScope{}, ErrPublicationForbidden
	}
	roles, err := a.live.ResolveLiveRoles(ctx, identity.EffectiveOrganizationID, identity.UserID)
	if err != nil {
		return PublicationScope{}, err
	}
	if !a.permissions.Authorize("", roles, authz.PermissionProductSourcingWrite) {
		return PublicationScope{}, ErrPublicationForbidden
	}
	return PublicationScope{OrganizationID: identity.EffectiveOrganizationID, ActorID: identity.UserID}, nil
}

var _ PublicationAuthorizer = (*ContextAuthorizer)(nil)
