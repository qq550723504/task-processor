package listingkit

import (
	"context"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"task-processor/internal/authz"
)

var (
	ownerScopeRequired     atomic.Bool
	ownerScopeRequiredTest sync.Mutex
)

type requestRolesContextKey struct{}

func ConfigureOwnerScopeRequired(required bool) {
	ownerScopeRequired.Store(required)
}

func SetOwnerScopeRequiredForTesting(required bool) func() {
	ownerScopeRequiredTest.Lock()
	previous := ownerScopeRequired.Load()
	ownerScopeRequired.Store(required)
	return func() {
		ownerScopeRequired.Store(previous)
		ownerScopeRequiredTest.Unlock()
	}
}

func OwnerScopeEnabled() bool {
	return ownerScopeRequired.Load()
}

func RequestUserIDFromContext(ctx context.Context) string {
	return strings.TrimSpace(RequestIdentityFromContext(ctx).UserID)
}

func WithRequestRoles(ctx context.Context, roles []string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	normalized := normalizeRequestRoles(roles)
	if len(normalized) == 0 {
		return ctx
	}
	return context.WithValue(ctx, requestRolesContextKey{}, normalized)
}

func RequestRolesFromContext(ctx context.Context) []string {
	if ctx == nil {
		return nil
	}
	roles, ok := ctx.Value(requestRolesContextKey{}).([]string)
	if !ok || len(roles) == 0 {
		return nil
	}
	return append([]string(nil), roles...)
}

func RequestHasPlatformAdminAccess(ctx context.Context) bool {
	return authz.IsListingKitPlatformAdmin(RequestUserIDFromContext(ctx), RequestRolesFromContext(ctx))
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

func normalizeRequestRoles(roles []string) []string {
	out := make([]string, 0, len(roles))
	for _, role := range roles {
		normalized := strings.TrimSpace(role)
		if normalized == "" || slices.Contains(out, normalized) {
			continue
		}
		out = append(out, normalized)
	}
	return out
}
