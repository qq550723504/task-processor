package listingkit

import (
	"context"
	"strings"
	"sync/atomic"

	openaiclient "task-processor/internal/infra/clients/openai"
)

var ownerScopeRequired atomic.Bool

func ConfigureOwnerScopeRequired(required bool) {
	ownerScopeRequired.Store(required)
}

func SetOwnerScopeRequiredForTesting(required bool) func() {
	previous := ownerScopeRequired.Load()
	ownerScopeRequired.Store(required)
	return func() {
		ownerScopeRequired.Store(previous)
	}
}

func OwnerScopeEnabled() bool {
	return ownerScopeRequired.Load()
}

func RequestUserIDFromContext(ctx context.Context) string {
	return strings.TrimSpace(openaiclient.IdentityFromContext(ctx).UserID)
}

func ResolveTaskUserID(task *Task) string {
	if task == nil {
		return ""
	}
	if value := strings.TrimSpace(task.UserID); value != "" {
		return value
	}
	if task.Request == nil {
		return ""
	}
	return strings.TrimSpace(task.Request.UserID)
}
