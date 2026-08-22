package aiidentity

import (
	"context"
	"strings"
)

// TenantIDFromContext returns the normalized verified tenant scope. An empty
// value means the caller is intentionally unscoped.
func TenantIDFromContext(ctx context.Context) string {
	return strings.TrimSpace(FromContext(ctx).TenantID)
}

// TenantMatchesContext reports whether the persisted execution tenant is
// accessible to the verified identity in ctx. An empty verified tenant keeps
// legacy worker and migration contexts unscoped.
func TenantMatchesContext(ctx context.Context, persistedTenantID string) bool {
	tenantID := TenantIDFromContext(ctx)
	return tenantID == "" || tenantID == strings.TrimSpace(persistedTenantID)
}
