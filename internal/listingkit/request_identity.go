package listingkit

import (
	"context"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
)

type RequestIdentity struct {
	TenantID string
	UserID   string
}

func WithRequestIdentity(ctx context.Context, identity RequestIdentity) context.Context {
	return openaiclient.WithIdentity(ctx, openaiclient.Identity{
		TenantID: identity.TenantID,
		UserID:   identity.UserID,
	})
}

func RequestIdentityFromContext(ctx context.Context) RequestIdentity {
	identity := openaiclient.IdentityFromContext(ctx)
	return RequestIdentity{
		TenantID: strings.TrimSpace(identity.TenantID),
		UserID:   strings.TrimSpace(identity.UserID),
	}
}
