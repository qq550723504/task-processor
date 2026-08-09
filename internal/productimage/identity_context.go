package productimage

import (
	"context"
	"strings"

	"task-processor/internal/shared/aiidentity"
)

type AIIdentity struct {
	TenantID string
	UserID   string
}

func AIIdentityFromContext(ctx context.Context) AIIdentity {
	identity := aiidentity.FromContext(ctx)
	return AIIdentity{TenantID: identity.TenantID, UserID: identity.UserID}
}

func WithAIIdentity(ctx context.Context, identity AIIdentity) context.Context {
	return aiidentity.WithIdentity(ctx, aiidentity.Identity{
		TenantID: strings.TrimSpace(identity.TenantID),
		UserID:   strings.TrimSpace(identity.UserID),
	})
}

func WithTaskIdentity(ctx context.Context, task *Task) context.Context {
	if task == nil {
		return ctx
	}
	identity := AIIdentity{TenantID: task.TenantID, UserID: task.UserID}
	if strings.TrimSpace(identity.TenantID) == "" && strings.TrimSpace(identity.UserID) == "" {
		return ctx
	}
	return WithAIIdentity(ctx, identity)
}
