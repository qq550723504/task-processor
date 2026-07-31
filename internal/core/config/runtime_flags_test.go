package config

import "testing"

func TestProductListingAPIRuntimeAutoMigrateEnabled(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_API_RUNTIME_AUTOMIGRATE", "")
	if !ProductListingAPIRuntimeAutoMigrateEnabled() {
		t.Fatal("expected auto-migrate to be enabled by default")
	}

	t.Setenv("TASK_PROCESSOR_API_RUNTIME_AUTOMIGRATE", "disabled")
	if ProductListingAPIRuntimeAutoMigrateEnabled() {
		t.Fatal("expected disabled to turn auto-migrate off")
	}
}
