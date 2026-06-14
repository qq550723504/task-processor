package submission

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStateFileKeepsTransitionSequencingBoundary(t *testing.T) {
	t.Parallel()

	path := filepath.Join("state.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	content := string(data)

	forbidden := []string{
		"&sheinpub.SubmissionRecord{",
		"ResponseOutcome{",
		"func responseOutcome(",
	}
	for _, pattern := range forbidden {
		if strings.Contains(content, pattern) {
			t.Fatalf("%s should not contain %q; pure SHEIN record/outcome shaping belongs in internal/publishing/shein", path, pattern)
		}
	}
}
