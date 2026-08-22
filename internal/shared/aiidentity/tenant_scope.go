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

// NormalizePersistedTenantID removes only portable ASCII U+0020 padding.
// Other whitespace is noncanonical and remains so tenant checks fail closed.
func NormalizePersistedTenantID(tenantID string) string {
	return strings.Trim(tenantID, " ")
}

// TenantMatchesContext reports whether the persisted execution tenant is
// accessible to the verified identity in ctx. An empty verified tenant keeps
// legacy worker and migration contexts unscoped.
func TenantMatchesContext(ctx context.Context, persistedTenantID string) bool {
	tenantID := TenantIDFromContext(ctx)
	return tenantID == "" || tenantID == NormalizePersistedTenantID(persistedTenantID)
}

// TenantCanCreateTask applies the stricter create boundary. Trusted worker and
// migration contexts remain unscoped, while a tenant-scoped caller may create
// only a task with a complete authoritative envelope for that tenant.
func TenantCanCreateTask(ctx context.Context, persisted PersistedExecutionEnvelope) bool {
	if TenantIDFromContext(ctx) == "" {
		return true
	}
	return persisted.State() == PersistedEnvelopePresent && TenantMatchesContext(ctx, persisted.ExecutionTenantID)
}
