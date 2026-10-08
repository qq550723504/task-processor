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
