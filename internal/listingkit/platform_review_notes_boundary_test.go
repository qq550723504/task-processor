package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestListingKitUsesCommonReviewNoteCollection(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"assembler.go", "temu_mapper.go", "walmart_mapper.go"} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(source)
		if !strings.Contains(content, "common.CollectReviewNotes(") {
			t.Fatalf("%s should use common.CollectReviewNotes", path)
		}
		if strings.Contains(content, "collectReviewNotes(") {
			t.Fatalf("%s should not use the ListingKit duplicate collectReviewNotes helper", path)
		}
	}

	helperSource, err := os.ReadFile("platform_helpers.go")
	if err != nil {
		t.Fatalf("read platform_helpers.go: %v", err)
	}
	if strings.Contains(string(helperSource), "func collectReviewNotes(") {
		t.Fatal("platform_helpers.go should not duplicate common review note collection")
	}
}
