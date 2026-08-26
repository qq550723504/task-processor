package listingkit

import (
	"context"
	"strings"

	"task-processor/internal/shared/aiidentity"
)

type RequestIdentity struct {
	TenantID string
	UserID   string
}

func WithRequestIdentity(ctx context.Context, identity RequestIdentity) context.Context {
	current := aiidentity.FromContext(ctx)
	current.TenantID = identity.TenantID
	current.UserID = identity.UserID
	return aiidentity.WithIdentity(ctx, current)
}

func RequestIdentityFromContext(ctx context.Context) RequestIdentity {
	identity := aiidentity.FromContext(ctx)
	return RequestIdentity{
		TenantID: strings.TrimSpace(identity.TenantID),
		UserID:   strings.TrimSpace(identity.UserID),
	}
}
