package listingkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetiredListingKitSubmissionPackageStaysAbsent(t *testing.T) {
	t.Parallel()

	retiredDir := filepath.Join("..", "..", "internal", "listingkit", "submission")
	if _, err := os.Stat(retiredDir); err == nil {
		t.Fatalf("internal/listingkit/submission directory still exists; keep SHEIN transition sequencing in shein_submit_state.go")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", retiredDir, err)
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
		t.Fatalf("%s imports retired internal/listingkit/submission compatibility package", path)
	}
}
