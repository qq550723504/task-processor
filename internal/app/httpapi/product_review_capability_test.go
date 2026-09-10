package httpapi

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"task-processor/internal/authidentity"
	"task-processor/internal/httproute"
	"task-processor/internal/product/review"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/workbenchcontext"
)

type productReviewResolverStub struct {
	policy httproute.OrganizationAccessPolicy
	input  workbenchcontext.ResolveInput
	value  authidentity.AuthenticatedIdentity
	err    error
}

func (stub *productReviewResolverStub) Resolve(_ context.Context, policy httproute.OrganizationAccessPolicy, input workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error) {
	stub.policy, stub.input = policy, input
	return stub.value, stub.err
}

func TestProductReviewRequestCapabilityPerformsFreshLiveWriteResolution(t *testing.T) {
	now := time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC)
	identity := authidentity.AuthenticatedIdentity{
		TenantID: "org", EffectiveOrganizationID: "org", HomeOrganizationID: "home", UserID: "actor",
		Roles: []string{"cached-role"}, OrganizationGrants: []authidentity.OrganizationGrant{{OrganizationID: "org", Roles: []string{"cached-role"}}},
		TokenExpiresAt: now.Add(time.Hour),
	}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), identity)
	ctx, err := (productReviewCapabilityBinder{now: func() time.Time { return now }}).Bind(ctx, "Bearer request-secret")
	require.NoError(t, err)

	resolver := &productReviewResolverStub{value: authidentity.AuthenticatedIdentity{
		TenantID: "org", EffectiveOrganizationID: "org", UserID: "actor", Roles: []string{"fresh-role"}, TokenExpiresAt: identity.TokenExpiresAt,
	}}
	roles, err := (&productReviewLiveOrganizationAccess{resolver: resolver, now: func() time.Time { return now }}).ResolveLiveRoles(ctx, "org", "actor")
	require.NoError(t, err)
	require.Equal(t, []string{"fresh-role"}, roles)
	require.Equal(t, httproute.OrganizationAccessPolicyLiveWrite, resolver.policy)
	require.Equal(t, "request-secret", resolver.input.BearerToken)
	require.Equal(t, "org", resolver.input.RequestedOrganizationID)
	require.Equal(t, "actor", resolver.input.Identity.UserID)
	require.Equal(t, "home", resolver.input.Identity.HomeOrganizationID)
	require.Nil(t, resolver.input.Identity.Roles)
	require.Nil(t, resolver.input.Identity.OrganizationGrants)
}

func TestProductReviewRequestCapabilityRejectsReplayOutsideBoundRequest(t *testing.T) {
	now := time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC)
	identity := authidentity.AuthenticatedIdentity{TenantID: "org", EffectiveOrganizationID: "org", UserID: "actor", TokenExpiresAt: now.Add(time.Hour)}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), identity)
	resolver := &productReviewResolverStub{}
	access := &productReviewLiveOrganizationAccess{resolver: resolver, now: func() time.Time { return now }}

	_, err := access.ResolveLiveRoles(ctx, "org", "actor")
	require.ErrorIs(t, err, sourcing.ErrPublicationForbidden)
	ctx, err = (productReviewCapabilityBinder{now: func() time.Time { return now }}).Bind(ctx, "Bearer request-secret")
	require.NoError(t, err)
	_, err = access.ResolveLiveRoles(ctx, "other", "actor")
	require.ErrorIs(t, err, sourcing.ErrPublicationForbidden)

	expired := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{
		TenantID: "org", EffectiveOrganizationID: "org", UserID: "actor", TokenExpiresAt: now,
	})
	_, err = (productReviewCapabilityBinder{now: func() time.Time { return now }}).Bind(expired, "Bearer request-secret")
	require.ErrorIs(t, err, review.ErrForbidden)
}

func TestProductReviewRequestCapabilityMapsLiveRevocation(t *testing.T) {
	now := time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC)
	identity := authidentity.AuthenticatedIdentity{TenantID: "org", EffectiveOrganizationID: "org", UserID: "actor", TokenExpiresAt: now.Add(time.Hour)}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), identity)
	ctx, err := (productReviewCapabilityBinder{now: func() time.Time { return now }}).Bind(ctx, "Bearer request-secret")
	require.NoError(t, err)
	resolver := &productReviewResolverStub{err: workbenchcontext.ErrOrganizationAccessRevoked}
	_, err = (&productReviewLiveOrganizationAccess{resolver: resolver, now: func() time.Time { return now }}).ResolveLiveRoles(ctx, "org", "actor")
	require.ErrorIs(t, err, sourcing.ErrPublicationForbidden)
}
