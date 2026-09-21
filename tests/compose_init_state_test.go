package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestComposeInitStateMarkerRejectsStaleAndAcceptsCurrent(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("POSIX shell is required to exercise the Compose initializer helper")
	}
	helper, err := filepath.Abs("../deployments/docker/compose-init-state.sh")
	if err != nil {
		t.Fatal(err)
	}
	quote := func(value string) string {
		return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
	}

	t.Run("rejects legacy marker", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), ".init-complete")
		if err := os.WriteFile(marker, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(sh, "-eu", "-c", fmt.Sprintf(
			". %s; require_current_init_marker %s current-v2",
			quote(helper), quote(marker),
		))
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("legacy marker was accepted, output=%s", output)
		}
		if !strings.Contains(string(output), "stale") {
			t.Fatalf("stale marker error = %s", output)
		}
	})

	t.Run("accepts and writes current marker", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), ".init-complete")
		cmd := exec.Command(sh, "-eu", "-c", fmt.Sprintf(
			". %s; write_current_init_marker %s current-v2; require_current_init_marker %s current-v2; test \"$(cat %s)\" = current-v2",
			quote(helper), quote(marker), quote(marker), quote(marker),
		))
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("current marker flow failed: %v, output=%s", err, output)
		}
	})
}
