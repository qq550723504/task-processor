package listingsubscription

import (
	"context"
	"strings"
)

// TenantDisplayNameResolver provides a dedicated layer for resolving
// operator-facing tenant display names without coupling the subscription
// domain to a specific tenant directory implementation.
type TenantDisplayNameResolver interface {
	ResolveTenantDisplayName(ctx context.Context, tenantID string) (string, error)
}

type fallbackTenantDisplayNameResolver struct{}

func (fallbackTenantDisplayNameResolver) ResolveTenantDisplayName(_ context.Context, tenantID string) (string, error) {
	return strings.TrimSpace(tenantID), nil
}
