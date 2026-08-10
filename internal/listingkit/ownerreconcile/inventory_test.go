package ownerreconcile

import (
	"strings"
	"testing"
)

func TestOwnerReconciliationInventoryIsFixedAndComplete(t *testing.T) {
	if len(ownerReconciliationInventory) != 24 {
		t.Fatalf("inventory length = %d, want 24", len(ownerReconciliationInventory))
	}
	seen := make(map[string]struct{}, len(ownerReconciliationInventory))
	for _, spec := range ownerReconciliationInventory {
		if _, exists := seen[spec.Table]; exists {
			t.Fatalf("duplicate inventory table %q", spec.Table)
		}
		seen[spec.Table] = struct{}{}
		if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(spec.Query)), "SELECT ") {
			t.Fatalf("inventory query for %s is not SELECT-only: %s", spec.Table, spec.Query)
		}
		if strings.Contains(spec.Query, "ai_client_credentials") {
			t.Fatalf("tenant-wide AI credentials must be excluded from reconciliation")
		}
		if err := validateTableSpec(spec); err != nil {
			t.Fatalf("inventory spec %s invalid: %v", spec.Table, err)
		}
	}
}
