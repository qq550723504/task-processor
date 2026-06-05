package httpapi

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/listingkit"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

func TestShouldAutoMigrateProductListingAPIRuntimeDefaultsTrue(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_API_RUNTIME_AUTOMIGRATE", "")

	if !shouldAutoMigrateProductListingAPIRuntime() {
		t.Fatal("expected product listing API runtime auto-migrate to default to true")
	}
}

func TestShouldAutoMigrateProductListingAPIRuntimeHonorsFalse(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_API_RUNTIME_AUTOMIGRATE", "false")

	if shouldAutoMigrateProductListingAPIRuntime() {
		t.Fatal("expected product listing API runtime auto-migrate to honor false")
	}
}

func TestAutoMigrateProductListingAPIRuntimeSchemaRejectsNilDB(t *testing.T) {
	t.Parallel()

	if err := AutoMigrateProductListingAPIRuntimeSchema(nil); err == nil {
		t.Fatal("expected nil db to fail")
	}
}

func TestTaskModelsUseDistinctTableNames(t *testing.T) {
	t.Parallel()

	namer := schema.NamingStrategy{}
	cache := &sync.Map{}

	productSchema, err := schema.Parse(&productenrich.Task{}, cache, namer)
	if err != nil {
		t.Fatalf("parse productenrich schema: %v", err)
	}
	imageSchema, err := schema.Parse(&productimage.Task{}, cache, namer)
	if err != nil {
		t.Fatalf("parse productimage schema: %v", err)
	}
	amazonSchema, err := schema.Parse(&amazonlisting.Task{}, cache, namer)
	if err != nil {
		t.Fatalf("parse amazonlisting schema: %v", err)
	}
	listingKitSchema, err := schema.Parse(&listingkit.Task{}, cache, namer)
	if err != nil {
		t.Fatalf("parse listingkit schema: %v", err)
	}

	got := map[string]string{
		"productenrich": productSchema.Table,
		"productimage":  imageSchema.Table,
		"amazonlisting": amazonSchema.Table,
		"listingkit":    listingKitSchema.Table,
	}

	want := map[string]string{
		"productenrich": "product_enrich_tasks",
		"productimage":  "product_image_tasks",
		"amazonlisting": "amazon_listing_tasks",
		"listingkit":    "listing_kit_tasks",
	}

	seen := make(map[string]string, len(got))
	for name, table := range got {
		if table != want[name] {
			t.Fatalf("%s table = %q, want %q", name, table, want[name])
		}
		if previous, ok := seen[table]; ok {
			t.Fatalf("%s and %s share table %q", previous, name, table)
		}
		seen[table] = name
	}
}
