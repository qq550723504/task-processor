package commercetoolauth

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/authidentity"
)

func TestContextPrincipalResolverUsesOnlyAuthenticatedIdentity(t *testing.T) {
	now := time.Date(2026, 9, 11, 1, 2, 3, 0, time.UTC)
	roles := []string{"listingkit_operator"}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{
		TenantID:                "org-1",
		EffectiveOrganizationID: "org-1",
		UserID:                  "user-1",
		Roles:                   roles,
		TokenExpiresAt:          now.Add(time.Minute),
	})
	roles[0] = "mutated-source"

	resolver := ContextPrincipalResolver{Now: func() time.Time { return now }}
	principal, err := resolver.ResolvePrincipal(ctx)
	if err != nil {
		t.Fatalf("ResolvePrincipal() error = %v", err)
	}
	if principal.TenantID != "org-1" || principal.UserID != "user-1" || len(principal.Roles) != 1 || principal.Roles[0] != "listingkit_operator" {
		t.Fatalf("principal = %#v", principal)
	}
	principal.Roles[0] = "mutated-result"
	again, err := resolver.ResolvePrincipal(ctx)
	if err != nil {
		t.Fatalf("ResolvePrincipal() again error = %v", err)
	}
	if again.Roles[0] != "listingkit_operator" {
		t.Fatalf("resolver leaked roles slice: %#v", again.Roles)
	}
}

func TestContextPrincipalResolverRejectsMissingTrustedIdentityFields(t *testing.T) {
	now := time.Date(2026, 9, 11, 1, 2, 3, 0, time.UTC)
	tests := []struct {
		name     string
		identity *authidentity.AuthenticatedIdentity
	}{
		{name: "missing identity"},
		{name: "missing tenant", identity: &authidentity.AuthenticatedIdentity{EffectiveOrganizationID: "org-1", UserID: "user-1", Roles: []string{"listingkit_operator"}, TokenExpiresAt: now.Add(time.Minute)}},
		{name: "missing effective organization", identity: &authidentity.AuthenticatedIdentity{TenantID: "org-1", UserID: "user-1", Roles: []string{"listingkit_operator"}, TokenExpiresAt: now.Add(time.Minute)}},
		{name: "mismatched effective organization", identity: &authidentity.AuthenticatedIdentity{TenantID: "org-1", EffectiveOrganizationID: "org-2", UserID: "user-1", Roles: []string{"listingkit_operator"}, TokenExpiresAt: now.Add(time.Minute)}},
		{name: "missing user", identity: &authidentity.AuthenticatedIdentity{TenantID: "org-1", EffectiveOrganizationID: "org-1", Roles: []string{"listingkit_operator"}, TokenExpiresAt: now.Add(time.Minute)}},
		{name: "missing roles", identity: &authidentity.AuthenticatedIdentity{TenantID: "org-1", EffectiveOrganizationID: "org-1", UserID: "user-1", TokenExpiresAt: now.Add(time.Minute)}},
		{name: "missing expiry", identity: &authidentity.AuthenticatedIdentity{TenantID: "org-1", EffectiveOrganizationID: "org-1", UserID: "user-1", Roles: []string{"listingkit_operator"}}},
		{name: "expired", identity: &authidentity.AuthenticatedIdentity{TenantID: "org-1", EffectiveOrganizationID: "org-1", UserID: "user-1", Roles: []string{"listingkit_operator"}, TokenExpiresAt: now}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.identity != nil {
				ctx = authidentity.WithAuthenticatedIdentity(ctx, *tt.identity)
			}
			if _, err := (ContextPrincipalResolver{Now: func() time.Time { return now }}).ResolvePrincipal(ctx); err == nil {
				t.Fatal("ResolvePrincipal() error = nil")
			}
		})
	}
}
