package tenantctx

import (
	"context"

	sharedtenantctx "task-processor/internal/shared/tenantctx"
)

const DefaultTenantID = sharedtenantctx.DefaultTenantID

func NormalizeTenantID(tenantID string) string {
	return sharedtenantctx.NormalizeTenantID(tenantID)
}

func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return sharedtenantctx.WithTenantID(ctx, tenantID)
}

func TenantIDFromContext(ctx context.Context) string {
	return sharedtenantctx.TenantIDFromContext(ctx)
}

func TenantScopeFromContext(ctx context.Context) (string, bool) {
	return sharedtenantctx.TenantScopeFromContext(ctx)
}

func MatchesTenant(recordTenantID, tenantID string) bool {
	return sharedtenantctx.MatchesTenant(recordTenantID, tenantID)
}
