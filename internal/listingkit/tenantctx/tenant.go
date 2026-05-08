package tenantctx

import (
	"context"
	"strings"
)

const DefaultTenantID = "default"

type contextKey struct{}

func NormalizeTenantID(tenantID string) string {
	if trimmed := strings.TrimSpace(tenantID); trimmed != "" {
		return trimmed
	}
	return DefaultTenantID
}

func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, contextKey{}, NormalizeTenantID(tenantID))
}

func TenantIDFromContext(ctx context.Context) string {
	if tenantID, ok := TenantScopeFromContext(ctx); ok {
		return tenantID
	}
	return DefaultTenantID
}

func TenantScopeFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	value, ok := ctx.Value(contextKey{}).(string)
	if !ok {
		return "", false
	}
	return NormalizeTenantID(value), true
}

func MatchesTenant(recordTenantID, tenantID string) bool {
	normalized := NormalizeTenantID(tenantID)
	record := strings.TrimSpace(recordTenantID)
	if normalized == DefaultTenantID {
		return record == "" || record == DefaultTenantID
	}
	return record == normalized
}
