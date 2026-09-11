package commercetoolauth

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/workbenchcontext"
)

func TestWorkbenchPrincipalResolverResolvesEveryInvocation(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	stub := &cachedReadOrganizationResolverStub{identity: authidentity.AuthenticatedIdentity{
		TenantID: "org-1", EffectiveOrganizationID: "org-1", UserID: "user-1",
		Roles: []string{"listingkit_operator"}, TokenExpiresAt: now.Add(time.Minute),
	}}
	resolver, err := NewWorkbenchPrincipalResolver(stub, func() time.Time { return now })
	if err != nil {
		t.Fatalf("NewWorkbenchPrincipalResolver(): %v", err)
	}
	ctx := WithOrganizationRequest(context.Background(), OrganizationRequest{
		Identity:    authidentity.AuthenticatedIdentity{UserID: "user-1", HomeOrganizationID: "org-home", TokenExpiresAt: now.Add(time.Minute)},
		BearerToken: "bearer", RequestedOrganizationID: "org-1",
	})

	for range 2 {
		principal, err := resolver.ResolvePrincipal(ctx)
		if err != nil || principal.TenantID != "org-1" || principal.UserID != "user-1" {
			t.Fatalf("ResolvePrincipal() = %#v, %v", principal, err)
		}
	}
	if stub.calls != 2 || stub.request.BearerToken != "bearer" || stub.request.RequestedOrganizationID != "org-1" {
		t.Fatalf("organization resolver calls=%d request=%#v", stub.calls, stub.request)
	}
}

func TestWorkbenchPrincipalResolverFailsClosedAfterGrantResolutionChanges(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	stub := &cachedReadOrganizationResolverStub{identity: authidentity.AuthenticatedIdentity{
		TenantID: "org-1", EffectiveOrganizationID: "org-1", UserID: "user-1",
		Roles: []string{"listingkit_operator"}, TokenExpiresAt: now.Add(time.Minute),
	}}
	resolver, _ := NewWorkbenchPrincipalResolver(stub, func() time.Time { return now })
	ctx := WithOrganizationRequest(context.Background(), OrganizationRequest{Identity: authidentity.AuthenticatedIdentity{UserID: "user-1", TokenExpiresAt: now.Add(time.Minute)}, BearerToken: "bearer", RequestedOrganizationID: "org-1"})
	if _, err := resolver.ResolvePrincipal(ctx); err != nil {
		t.Fatalf("first ResolvePrincipal(): %v", err)
	}
	stub.err = workbenchcontext.ErrOrganizationAccessRevoked
	if _, err := resolver.ResolvePrincipal(ctx); !errors.Is(err, workbenchcontext.ErrOrganizationAccessRevoked) {
		t.Fatalf("second ResolvePrincipal() error = %v", err)
	}
	if stub.calls != 2 {
		t.Fatalf("organization resolver calls = %d", stub.calls)
	}
}

func TestWorkbenchPrincipalResolverRejectsMissingRequestAndExpiredResolvedIdentity(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	stub := &cachedReadOrganizationResolverStub{identity: authidentity.AuthenticatedIdentity{TenantID: "org-1", EffectiveOrganizationID: "org-1", UserID: "user-1", Roles: []string{"listingkit_operator"}, TokenExpiresAt: now}}
	resolver, _ := NewWorkbenchPrincipalResolver(stub, func() time.Time { return now })
	if _, err := resolver.ResolvePrincipal(context.Background()); err == nil {
		t.Fatal("missing request error = nil")
	}
	ctx := WithOrganizationRequest(context.Background(), OrganizationRequest{BearerToken: "bearer"})
	if _, err := resolver.ResolvePrincipal(ctx); err == nil {
		t.Fatal("expired resolved identity error = nil")
	}
}

type cachedReadOrganizationResolverStub struct {
	identity authidentity.AuthenticatedIdentity
	err      error
	calls    int
	request  OrganizationRequest
}

func (s *cachedReadOrganizationResolverStub) ResolveCachedReadOrganization(_ context.Context, request OrganizationRequest) (authidentity.AuthenticatedIdentity, error) {
	s.calls++
	s.request = request
	return s.identity, s.err
}
