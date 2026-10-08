package collection

import (
	"context"
	"task-processor/internal/authidentity"
	"time"
)

// AuthorizedOwner grants no product identity. A retained selection owner must
// separately load its exact actor/member-qualified durable source reference.
type AuthorizedOwner struct{ proof AuthorizedSelection }

func (p AuthorizedOwner) Scope(ctx context.Context) (Scope, error) { return p.proof.Scope(ctx) }
func (s *Service) AuthorizeOwner(ctx context.Context) (AuthorizedOwner, error) {
	if ctx == nil {
		return AuthorizedOwner{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorize(ctx, PermissionRead)
	if err != nil {
		return AuthorizedOwner{}, err
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || scope.Validate() != nil || identity.TenantID != scope.OrganizationID || identity.EffectiveOrganizationID != scope.OrganizationID || identity.UserID != scope.ActorID || identity.EffectiveMemberID != scope.MemberID || !time.Now().Before(identity.TokenExpiresAt) {
		return AuthorizedOwner{}, ErrForbidden
	}
	expires := time.Now().Add(5 * time.Second)
	if deadline, _ := ctx.Deadline(); deadline.Before(expires) {
		expires = deadline
	}
	if identity.TokenExpiresAt.Before(expires) {
		expires = identity.TokenExpiresAt
	}
	return AuthorizedOwner{proof: AuthorizedSelection{scope: scope, expiresAt: expires}}, nil
}
