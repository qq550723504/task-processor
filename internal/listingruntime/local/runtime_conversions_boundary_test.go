package local

import (
	"os"
	"strings"
	"testing"

	"task-processor/internal/listingadmin"
)

func TestRuntimeConversionsUseListingAdminDTONames(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"runtime_conversions.go", "local_runtime_adapter.go"} {
		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		source := string(content)
		for _, marker := range []string{
			"FromManagement",
			"fromManagement",
			"managementProductImportMappingFromRuntime",
		} {
			if strings.Contains(source, marker) {
				t.Fatalf("%s mentions %q; local listing runtime conversions should use listing-admin DTO naming instead of retired management-service naming", name, marker)
			}
		}
	}

	conversions, err := os.ReadFile("runtime_conversions.go")
	if err != nil {
		t.Fatalf("read runtime_conversions.go: %v", err)
	}
	source := string(conversions)
	for _, marker := range []string{
		"runtimeStoreFromListingAdminDTO",
		"runtimePauseDetailFromListingAdminDTO",
		"runtimeOperationStrategyFromListingAdminDTO",
		"runtimeProductImportMappingFromListingAdminDTO",
		"listingAdminProductImportMappingCreateDTOFromRuntime",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("runtime_conversions.go should contain %q", marker)
		}
	}
}

func TestRuntimeStoreFromListingPreservesOwner(t *testing.T) {
	t.Parallel()

	store := runtimeStoreFromListing(&listingadmin.Store{ID: 7, OwnerUserID: "zitadel-sub-1"})
	if store == nil || store.OwnerUserID != "zitadel-sub-1" {
		t.Fatalf("runtime store owner = %q, want zitadel-sub-1", store.OwnerUserID)
	}
}
