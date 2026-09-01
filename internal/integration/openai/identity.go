package openai

import (
	"context"
	"strings"

	"task-processor/internal/shared/aiidentity"
)

// Identity identifies the tenant/user whose AI credentials should be used.
// User-level clients take precedence over tenant-level clients.
type Identity struct {
	TenantID string
	UserID   string
}

func WithIdentity(ctx context.Context, identity Identity) context.Context {
	identity.TenantID = strings.TrimSpace(identity.TenantID)
	identity.UserID = strings.TrimSpace(identity.UserID)
	current := aiidentity.FromContext(ctx)
	current.TenantID = identity.TenantID
	current.UserID = identity.UserID
	return aiidentity.WithIdentity(ctx, current)
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
	identity := aiidentity.FromContext(ctx)
	return Identity{TenantID: identity.TenantID, UserID: identity.UserID}
}
