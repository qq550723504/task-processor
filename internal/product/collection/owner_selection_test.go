package collection

import (
	"context"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"testing"
	"time"
)

func TestRetainedSourceOwnerProofStillRequiresCurrentDataReadAndOriginalMembership(t *testing.T) {
	scope := Scope{"org-a", "actor-a", "member-a"}
	authorization := &testAuth{scope: scope}
	service, err := NewService(selectionStore{}, authorization, testSources{})
	require.NoError(t, err)
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: scope.OrganizationID, EffectiveOrganizationID: scope.OrganizationID, UserID: scope.ActorID, EffectiveMemberID: scope.MemberID, TokenExpiresAt: time.Now().Add(time.Minute)})
	proof, err := service.AuthorizeOwner(ctx)
	require.NoError(t, err)
	got, err := proof.Scope(ctx)
	require.NoError(t, err)
	require.Equal(t, scope, got)
	require.Equal(t, []string{PermissionRead}, authorization.calls)
	other := authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{TenantID: scope.OrganizationID, EffectiveOrganizationID: scope.OrganizationID, UserID: scope.ActorID, EffectiveMemberID: "new-member", TokenExpiresAt: time.Now().Add(time.Minute)})
	_, err = proof.Scope(other)
	require.ErrorIs(t, err, ErrForbidden)
	_, err = (AuthorizedOwner{}).Scope(ctx)
	require.ErrorIs(t, err, ErrForbidden)
	authorization.err = ErrForbidden
	_, err = service.AuthorizeOwner(ctx)
	require.ErrorIs(t, err, ErrForbidden)
}

type executionOwnerAuth struct {
	scope  Scope
	denied bool
}

func (a *executionOwnerAuth) AuthorizeExecution(_ context.Context, scope Scope, permission string) error {
	if a.denied || scope != a.scope || permission != PermissionRead {
		return ErrForbidden
	}
	return nil
}
func TestExecutionOwnerProofUsesLiveOriginalMembershipWithoutRequestIdentity(t *testing.T) {
	scope := Scope{"org-a", "actor-a", "member-a"}
	current := &executionOwnerAuth{scope: scope}
	owner := ExecutionOwnerAuthority{Authorization: current}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	proof, err := owner.AuthorizeExecutionOwner(ctx, scope)
	require.NoError(t, err)
	_, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	require.False(t, ok, "durable work does not forge a JWT identity")
	got, err := proof.Scope(ctx)
	require.NoError(t, err)
	require.Equal(t, scope, got)
	current.denied = true
	_, err = proof.Scope(ctx)
	require.ErrorIs(t, err, ErrForbidden, "revocation after mint must stop consumption")
	current.denied = false
	_, err = owner.AuthorizeExecutionOwner(ctx, Scope{scope.OrganizationID, scope.ActorID, "new-member"})
	require.ErrorIs(t, err, ErrForbidden)
	proof.execution.expiresAt = time.Now().Add(-time.Second)
	_, err = proof.Scope(ctx)
	require.ErrorIs(t, err, ErrForbidden)
	_, err = owner.AuthorizeExecutionOwner(context.Background(), scope)
	require.ErrorIs(t, err, ErrForbidden, "execution authority requires a bounded activity context")
}
