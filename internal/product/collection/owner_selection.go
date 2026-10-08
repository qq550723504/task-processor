package collection

import (
	"context"
	"task-processor/internal/authidentity"
	"time"
)

// AuthorizedOwner grants no product identity. A retained selection owner must
// separately load its exact actor/member-qualified durable source reference.
type AuthorizedOwner struct {
	proof     AuthorizedSelection
	execution *executionOwner
}

func (p AuthorizedOwner) Scope(ctx context.Context) (Scope, error) {
	if ctx == nil || ctx.Err() != nil {
		return Scope{}, ErrForbidden
	}
	if p.execution == nil {
		return p.proof.Scope(ctx)
	}
	e := p.execution
	if e.authorization == nil || e.scope.Validate() != nil || !time.Now().Before(e.expiresAt) {
		return Scope{}, ErrForbidden
	}
	if identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx); ok && (identity.TenantID != e.scope.OrganizationID || identity.EffectiveOrganizationID != e.scope.OrganizationID || identity.UserID != e.scope.ActorID || identity.EffectiveMemberID != e.scope.MemberID) {
		return Scope{}, ErrForbidden
	}
	if _, bounded := ctx.Deadline(); !bounded {
		return Scope{}, ErrForbidden
	}
	if err := e.authorization.AuthorizeExecution(ctx, e.scope, PermissionRead); err != nil || ctx.Err() != nil || !time.Now().Before(e.expiresAt) {
		return Scope{}, ErrForbidden
	}
	return e.scope, nil
}

// ExecutionAuthorizer is explicitly injected in the current worker assembly.
// It checks the original durable membership using live IAM and role policy.
type ExecutionAuthorizer interface {
	AuthorizeExecution(context.Context, Scope, string) error
}
type ExecutionOwnerAuthority struct{ Authorization ExecutionAuthorizer }
type executionOwner struct {
	scope         Scope
	expiresAt     time.Time
	authorization ExecutionAuthorizer
}

func (a ExecutionOwnerAuthority) AuthorizeExecutionOwner(ctx context.Context, scope Scope) (AuthorizedOwner, error) {
	if ctx == nil || ctx.Err() != nil || a.Authorization == nil || scope.Validate() != nil {
		return AuthorizedOwner{}, ErrForbidden
	}
	deadline, bounded := ctx.Deadline()
	if !bounded {
		return AuthorizedOwner{}, ErrForbidden
	}
	if err := a.Authorization.AuthorizeExecution(ctx, scope, PermissionRead); err != nil {
		return AuthorizedOwner{}, err
	}
	expiresAt := time.Now().Add(5 * time.Second)
	if deadline.Before(expiresAt) {
		expiresAt = deadline
	}
	proof := AuthorizedOwner{execution: &executionOwner{scope: scope, expiresAt: expiresAt, authorization: a.Authorization}}
	if _, err := proof.Scope(ctx); err != nil {
		return AuthorizedOwner{}, err
	}
	return proof, nil
}
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
