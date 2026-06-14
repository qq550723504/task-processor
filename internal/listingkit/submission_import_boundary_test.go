package listingkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListingKitSubmissionImportsStayWithinSheinAdapterBoundary(t *testing.T) {
	t.Parallel()

	allowed := map[string]struct{}{
		"shein_submit_state.go": {},
	}

	matches, err := filepath.Glob(filepath.Join("..", "..", "internal", "listingkit", "*.go"))
	if err != nil {
		t.Fatalf("glob listingkit files: %v", err)
	}
	for _, path := range matches {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		content := string(data)
		if !strings.Contains(content, "\"task-processor/internal/listingkit/submission\"") {
			continue
		}
		base := filepath.Base(path)
		if _, ok := allowed[base]; !ok {
			t.Fatalf("%s imports internal/listingkit/submission outside the approved SHEIN adapter boundary", path)
		}
	}
}
