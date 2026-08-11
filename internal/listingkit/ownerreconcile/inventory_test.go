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
		if spec.TenantDomain == TenantDomainLegacyNumeric && len(spec.CandidateColumns) > 0 {
			if !strings.Contains(spec.Query, "created_by") || !strings.Contains(spec.UpdateQuery, "created_by") {
				t.Fatalf("legacy inventory spec %s omits created_by candidate handling", spec.Table)
			}
		}
	}
}

func TestInventoryReturnsDefensiveCopy(t *testing.T) {
	first := Inventory()
	if len(first) == 0 {
		t.Fatal("expected fixed inventory")
	}
	original := first[0].Table
	first[0].Table = "mutated"
	second := Inventory()
	if second[0].Table != original {
		t.Fatalf("inventory was mutated through returned slice: %q", second[0].Table)
	}
}
