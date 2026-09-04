package httpapi

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/listingkit"
)

func TestCurrentTaskModelsUseDistinctTableNames(t *testing.T) {
	t.Parallel()

	namer := schema.NamingStrategy{}
	cache := &sync.Map{}

	amazonSchema, err := schema.Parse(&amazonlisting.Task{}, cache, namer)
	if err != nil {
		t.Fatalf("parse amazonlisting schema: %v", err)
	}
	listingKitSchema, err := schema.Parse(&listingkit.Task{}, cache, namer)
	if err != nil {
		t.Fatalf("parse listingkit schema: %v", err)
	}

	got := map[string]string{
		"amazonlisting": amazonSchema.Table,
		"listingkit":    listingKitSchema.Table,
	}

	want := map[string]string{
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
