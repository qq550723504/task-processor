package sourcing

import (
	"context"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
)

const publicationReadProofTTL = 5 * time.Second

type publicationReadProofContextKey struct{}

type publicationReadProof struct {
	actorID                 string
	tenantID                string
	effectiveOrganizationID string
	identityExpiresAt       time.Time
	proofExpiresAt          time.Time
	requestDeadline         time.Time
}

type readProofAuthorizer struct{ now func() time.Time }

// NewReadProofAuthorizer creates the Sourcing-owned verifier used only by a
// caller-transaction-bound reader. Callers cannot construct or persist proofs.
func NewReadProofAuthorizer() PublicationAuthorizer {
	return &readProofAuthorizer{now: time.Now}
}

func withPublicationReadProof(ctx context.Context, scope PublicationScope, now time.Time) (context.Context, error) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	deadline, hasDeadline := ctx.Deadline()
	if !ok || !hasDeadline || !authidentity.IsBoundedIdentifier(scope.ActorID) ||
		!authidentity.IsBoundedIdentifier(scope.OrganizationID) ||
		identity.UserID != scope.ActorID || identity.TenantID != scope.OrganizationID ||
		identity.EffectiveOrganizationID != scope.OrganizationID || identity.TokenExpiresAt.IsZero() ||
		!now.Before(identity.TokenExpiresAt) || !now.Before(deadline) {
		return ctx, ErrPublicationForbidden
	}
	expiresAt := now.Add(publicationReadProofTTL)
	if identity.TokenExpiresAt.Before(expiresAt) {
		expiresAt = identity.TokenExpiresAt
	}
	if deadline.Before(expiresAt) {
		expiresAt = deadline
	}
	proof := publicationReadProof{
		actorID: scope.ActorID, tenantID: identity.TenantID,
		effectiveOrganizationID: identity.EffectiveOrganizationID,
		identityExpiresAt:       identity.TokenExpiresAt, proofExpiresAt: expiresAt,
		requestDeadline: deadline,
	}
	return context.WithValue(ctx, publicationReadProofContextKey{}, proof), nil
}

func (a *readProofAuthorizer) Authorize(ctx context.Context) (PublicationScope, error) {
	if ctx == nil {
		return PublicationScope{}, ErrPublicationForbidden
	}
	if err := ctx.Err(); err != nil {
		return PublicationScope{}, err
	}
	proof, ok := ctx.Value(publicationReadProofContextKey{}).(publicationReadProof)
	identity, authenticated := authidentity.AuthenticatedIdentityFromContext(ctx)
	deadline, hasDeadline := ctx.Deadline()
	now := time.Now
	if a != nil && a.now != nil {
		now = a.now
	}
	current := now()
	if !ok || !authenticated || !hasDeadline ||
		identity.UserID != proof.actorID || identity.TenantID != proof.tenantID ||
		identity.EffectiveOrganizationID != proof.effectiveOrganizationID ||
		proof.tenantID != proof.effectiveOrganizationID ||
		!identity.TokenExpiresAt.Equal(proof.identityExpiresAt) ||
		deadline.After(proof.requestDeadline) || !current.Before(deadline) ||
		!current.Before(identity.TokenExpiresAt) || !current.Before(proof.proofExpiresAt) {
		return PublicationScope{}, ErrPublicationForbidden
	}
	return PublicationScope{OrganizationID: proof.effectiveOrganizationID, ActorID: proof.actorID}, nil
}

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
	if !a.permissions.Authorize(identity.UserID, roles, authz.PermissionProductSourcingWrite) {
		return PublicationScope{}, ErrPublicationForbidden
	}
	return PublicationScope{OrganizationID: identity.EffectiveOrganizationID, ActorID: identity.UserID}, nil
}

var _ PublicationAuthorizer = (*ContextAuthorizer)(nil)
