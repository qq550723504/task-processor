package listingadmin

import (
	"context"
	"slices"
	"strings"
	"sync/atomic"

	"task-processor/internal/authz"
	"gorm.io/gorm"
)

type requestUserIDContextKey struct{}
type requestRolesContextKey struct{}

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

func ownerScopeEnabled() bool {
	return ownerScopeRequired.Load()
}

func requestUserIDHeader(value string) string {
	return strings.TrimSpace(value)
}

func withRequestUserID(ctx context.Context, userID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	userID = requestUserIDHeader(userID)
	if userID == "" {
		return ctx
	}
	return context.WithValue(ctx, requestUserIDContextKey{}, userID)
}

func requestUserIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value, ok := ctx.Value(requestUserIDContextKey{}).(string); ok {
		return requestUserIDHeader(value)
	}
	return ""
}

func withRequestIdentity(ctx context.Context, userID string, roles []string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = withRequestUserID(ctx, userID)
	normalized := normalizeRequestRoles(roles)
	if len(normalized) == 0 {
		return ctx
	}
	return context.WithValue(ctx, requestRolesContextKey{}, normalized)
}

func requestRolesFromContext(ctx context.Context) []string {
	if ctx == nil {
		return nil
	}
	roles, ok := ctx.Value(requestRolesContextKey{}).([]string)
	if !ok || len(roles) == 0 {
		return nil
	}
	return append([]string(nil), roles...)
}

func requestHasPlatformAdminAccess(ctx context.Context) bool {
	return authz.IsListingKitPlatformAdmin(requestUserIDFromContext(ctx), requestRolesFromContext(ctx))
}

func applyOwnerScope(db *gorm.DB, ctx context.Context, ownerColumn string) *gorm.DB {
	if db == nil || !ownerScopeEnabled() {
		return db
	}
	if requestHasPlatformAdminAccess(ctx) {
		return db
	}
	ownerUserID := requestUserIDFromContext(ctx)
	if ownerUserID == "" || strings.TrimSpace(ownerColumn) == "" {
		return db
	}
	return db.Where(strings.TrimSpace(ownerColumn)+" = ?", ownerUserID)
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
