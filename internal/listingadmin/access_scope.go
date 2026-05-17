package listingadmin

import (
	"context"
	"strings"
	"sync/atomic"

	"gorm.io/gorm"
)

type requestUserIDContextKey struct{}

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

func applyOwnerScope(db *gorm.DB, ctx context.Context, ownerColumn string) *gorm.DB {
	if db == nil || !ownerScopeEnabled() {
		return db
	}
	ownerUserID := requestUserIDFromContext(ctx)
	if ownerUserID == "" || strings.TrimSpace(ownerColumn) == "" {
		return db
	}
	return db.Where(strings.TrimSpace(ownerColumn)+" = ?", ownerUserID)
}
