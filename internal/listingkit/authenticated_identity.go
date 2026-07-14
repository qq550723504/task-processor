package listingkit

import (
	"context"
	"strings"
)

type authenticatedIdentityContextKey struct{}

// AuthenticatedIdentity identifies the tenant, user, and roles verified for a request.
type AuthenticatedIdentity struct {
	TenantID string
	UserID   string
	Roles    []string
}

// WithAuthenticatedIdentity attaches a normalized verified identity to ctx.
func WithAuthenticatedIdentity(ctx context.Context, identity AuthenticatedIdentity) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	identity.TenantID = strings.TrimSpace(identity.TenantID)
	identity.UserID = strings.TrimSpace(identity.UserID)
	identity.Roles = append([]string(nil), identity.Roles...)
	return context.WithValue(ctx, authenticatedIdentityContextKey{}, identity)
}

// AuthenticatedIdentityFromContext returns the verified identity stored in ctx.
func AuthenticatedIdentityFromContext(ctx context.Context) (AuthenticatedIdentity, bool) {
	if ctx == nil {
		return AuthenticatedIdentity{}, false
	}
	identity, ok := ctx.Value(authenticatedIdentityContextKey{}).(AuthenticatedIdentity)
	if !ok || strings.TrimSpace(identity.TenantID) == "" {
		return AuthenticatedIdentity{}, false
	}
	identity.TenantID = strings.TrimSpace(identity.TenantID)
	identity.UserID = strings.TrimSpace(identity.UserID)
	identity.Roles = append([]string(nil), identity.Roles...)
	return identity, true
}
