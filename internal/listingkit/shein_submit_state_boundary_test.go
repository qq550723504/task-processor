package listingkit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSheinSubmitStateKeepsTransitionSequencingBoundary(t *testing.T) {
	t.Parallel()

	path := filepath.Join("shein_submit_state.go")
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s should not exist; SHEIN submission state transitions belong in internal/publishing/shein", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}
