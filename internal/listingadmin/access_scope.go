package listingadmin

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"gorm.io/gorm"
	"task-processor/internal/authz"
	openaiclient "task-processor/internal/infra/clients/openai"
)

type requestUserIDContextKey struct{}
type requestRolesContextKey struct{}

var (
	ownerScopeRequired     atomic.Bool
	ownerScopeRequiredTest sync.Mutex
)

func init() {
	ownerScopeRequired.Store(true)
}

// EnableOwnerScope ensures the production owner-filtering invariant is active.
func EnableOwnerScope() {
	ownerScopeRequired.Store(true)
}

// SetOwnerScopeRequiredForTesting is reserved for tests, including external-package tests.
func SetOwnerScopeRequiredForTesting(required bool) func() {
	ownerScopeRequiredTest.Lock()
	previous := ownerScopeRequired.Load()
	ownerScopeRequired.Store(required)
	return func() {
		ownerScopeRequired.Store(previous)
		ownerScopeRequiredTest.Unlock()
	}
}

func ownerScopeEnabled() bool {
	return ownerScopeRequired.Load()
}

func OwnerScopeEnabled() bool {
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

// WithRequestRoles bridges verified application roles into the listing-admin
// access scope used by repository reads.
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

// RequestRolesFromContext returns the roles available to listing-admin access checks.
func RequestRolesFromContext(ctx context.Context) []string {
	return requestRolesFromContext(ctx)
}

func requestUserIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value, ok := ctx.Value(requestUserIDContextKey{}).(string); ok {
		return requestUserIDHeader(value)
	}
	return requestUserIDHeader(openaiclient.IdentityFromContext(ctx).UserID)
}

var ErrOwnerUserIDRequired = errors.New("owner user id is required")

// WithOwnerUserID supplies a trusted owner identity to internal write paths
// that do not originate from an HTTP request.
func WithOwnerUserID(ctx context.Context, ownerUserID string) context.Context {
	return withRequestUserID(ctx, ownerUserID)
}

func requireOwnerUserID(ctx context.Context, explicitOwner string) (string, error) {
	if owner := strings.TrimSpace(requestUserIDFromContext(ctx)); owner != "" {
		return owner, nil
	}
	if owner := strings.TrimSpace(explicitOwner); owner != "" {
		return owner, nil
	}
	return "", ErrOwnerUserIDRequired
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
	return WithRequestRoles(ctx, normalized)
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

func requestHasTenantAdminAccess(ctx context.Context) bool {
	return authz.IsListingKitTenantAdmin(requestUserIDFromContext(ctx), requestRolesFromContext(ctx))
}

func applyOwnerScope(db *gorm.DB, ctx context.Context, ownerColumn string) *gorm.DB {
	return applyOwnerScopeForUser(db, ctx, requestUserIDFromContext(ctx), ownerColumn)
}

func applyOwnerScopeForUser(db *gorm.DB, ctx context.Context, ownerUserID, ownerColumn string) *gorm.DB {
	if db == nil || !ownerScopeEnabled() {
		return db
	}
	if requestHasTenantAdminAccess(ctx) {
		return db
	}
	ownerUserID = strings.TrimSpace(ownerUserID)
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
