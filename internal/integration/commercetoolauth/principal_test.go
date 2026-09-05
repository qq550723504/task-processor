package commercetoolauth

import (
	"context"
	"testing"

	"task-processor/internal/authidentity"
)

func TestContextPrincipalResolverUsesOnlyAuthenticatedIdentity(t *testing.T) {
	roles := []string{"listingkit_operator"}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{
		TenantID: "tenant-1",
		UserID:   "user-1",
		Roles:    roles,
	})
	roles[0] = "mutated-source"

	principal, err := (ContextPrincipalResolver{}).ResolvePrincipal(ctx)
	if err != nil {
		t.Fatalf("ResolvePrincipal() error = %v", err)
	}
	if principal.TenantID != "tenant-1" || principal.UserID != "user-1" || len(principal.Roles) != 1 || principal.Roles[0] != "listingkit_operator" {
		t.Fatalf("principal = %#v", principal)
	}
	principal.Roles[0] = "mutated-result"
	again, err := (ContextPrincipalResolver{}).ResolvePrincipal(ctx)
	if err != nil {
		t.Fatalf("ResolvePrincipal() again error = %v", err)
	}
	if again.Roles[0] != "listingkit_operator" {
		t.Fatalf("resolver leaked roles slice: %#v", again.Roles)
	}
}

func TestContextPrincipalResolverRejectsMissingTrustedIdentityFields(t *testing.T) {
	tests := []struct {
		name     string
		identity *authidentity.AuthenticatedIdentity
	}{
		{name: "missing identity"},
		{name: "missing tenant", identity: &authidentity.AuthenticatedIdentity{UserID: "user-1", Roles: []string{"listingkit_operator"}}},
		{name: "missing user", identity: &authidentity.AuthenticatedIdentity{TenantID: "tenant-1", Roles: []string{"listingkit_operator"}}},
		{name: "missing roles", identity: &authidentity.AuthenticatedIdentity{TenantID: "tenant-1", UserID: "user-1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.identity != nil {
				ctx = authidentity.WithAuthenticatedIdentity(ctx, *tt.identity)
			}
			if _, err := (ContextPrincipalResolver{}).ResolvePrincipal(ctx); err == nil {
				t.Fatal("ResolvePrincipal() error = nil")
			}
		})
	}
}
