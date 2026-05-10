package openai

import (
	"context"
	"strings"
)

type identityContextKey struct{}

// Identity identifies the tenant/user whose AI credentials should be used.
// User-level clients take precedence over tenant-level clients.
type Identity struct {
	TenantID string
	UserID   string
}

func WithIdentity(ctx context.Context, identity Identity) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	identity.TenantID = strings.TrimSpace(identity.TenantID)
	identity.UserID = strings.TrimSpace(identity.UserID)
	if identity.TenantID == "" && identity.UserID == "" {
		return ctx
	}
	return context.WithValue(ctx, identityContextKey{}, identity)
}

func WithTenantID(ctx context.Context, tenantID string) context.Context {
	identity := IdentityFromContext(ctx)
	identity.TenantID = strings.TrimSpace(tenantID)
	return WithIdentity(ctx, identity)
}

func WithUserID(ctx context.Context, userID string) context.Context {
	identity := IdentityFromContext(ctx)
	identity.UserID = strings.TrimSpace(userID)
	return WithIdentity(ctx, identity)
}

func IdentityFromContext(ctx context.Context) Identity {
	if ctx == nil {
		return Identity{}
	}
	if identity, ok := ctx.Value(identityContextKey{}).(Identity); ok {
		identity.TenantID = strings.TrimSpace(identity.TenantID)
		identity.UserID = strings.TrimSpace(identity.UserID)
		return identity
	}
	return Identity{}
}
