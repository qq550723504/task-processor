package productimage

import (
	"context"
	"strings"

	"task-processor/internal/shared/aiidentity"
)

type AIIdentity struct {
	TenantID       string
	UserID         string
	BusinessTaskID string
	TraceID        string
}

func AIIdentityFromContext(ctx context.Context) AIIdentity {
	identity := aiidentity.FromContext(ctx)
	return AIIdentity{TenantID: identity.TenantID, UserID: identity.UserID, BusinessTaskID: identity.BusinessTaskID, TraceID: identity.TraceID}
}

func WithAIIdentity(ctx context.Context, identity AIIdentity) context.Context {
	return aiidentity.WithIdentity(ctx, aiidentity.Identity{
		TenantID:       strings.TrimSpace(identity.TenantID),
		UserID:         strings.TrimSpace(identity.UserID),
		BusinessTaskID: strings.TrimSpace(identity.BusinessTaskID),
		TraceID:        strings.TrimSpace(identity.TraceID),
	})
}

func WithTaskIdentity(ctx context.Context, task *Task) context.Context {
	if task == nil {
		return ctx
	}
	identity := AIIdentityFromContext(ctx)
	identity.TenantID = strings.TrimSpace(task.TenantID)
	identity.UserID = strings.TrimSpace(task.UserID)
	identity.BusinessTaskID = strings.TrimSpace(task.ID)
	if identity.TenantID == "" && identity.UserID == "" && identity.BusinessTaskID == "" {
		return ctx
	}
	return WithAIIdentity(ctx, identity)
}
