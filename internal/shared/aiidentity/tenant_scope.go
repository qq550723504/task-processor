package aiidentity

import (
	"context"
	"strings"
)

// TenantMatchesContext reports whether the persisted execution tenant is
// accessible to the verified identity in ctx. An empty verified tenant keeps
// legacy worker and migration contexts unscoped.
func TenantMatchesContext(ctx context.Context, persistedTenantID string) bool {
	tenantID := FromContext(ctx).TenantID
	return tenantID == "" || tenantID == strings.TrimSpace(persistedTenantID)
}
