package aiidentity

import (
	"context"
	"strings"
)

// Identity is the verified tenant/user scope used for tenant-aware AI calls.
// It is provider-neutral so queue workers and domain packages do not depend on
// a concrete model client package just to carry request identity.
type Identity struct {
	TenantID       string
	UserID         string
	BusinessTaskID string
	TraceID        string
}

type contextKey struct{}

func WithIdentity(ctx context.Context, identity Identity) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	identity.TenantID = strings.TrimSpace(identity.TenantID)
	identity.UserID = strings.TrimSpace(identity.UserID)
	identity.BusinessTaskID = strings.TrimSpace(identity.BusinessTaskID)
	identity.TraceID = strings.TrimSpace(identity.TraceID)
	return context.WithValue(ctx, contextKey{}, identity)
}

func FromContext(ctx context.Context) Identity {
	if ctx == nil {
		return Identity{}
	}
	identity, _ := ctx.Value(contextKey{}).(Identity)
	identity.TenantID = strings.TrimSpace(identity.TenantID)
	identity.UserID = strings.TrimSpace(identity.UserID)
	identity.BusinessTaskID = strings.TrimSpace(identity.BusinessTaskID)
	identity.TraceID = strings.TrimSpace(identity.TraceID)
	return identity
}
