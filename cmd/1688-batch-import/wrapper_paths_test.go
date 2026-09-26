package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestOperationalWrapperResolvesPathsBeforeChangingDirectory is the twelfth review
// round's finding. The wrapper builds the binary from the repository root and therefore
// Push-Locations there, but -queue, -browser, -extension and -profile are paths the
// caller gives relative to where THEY are standing. Without resolving them first, the
// repository root silently becomes the base: the run would create or read a different
// queue, open a different browser profile, or fail to find an input that exists.
//
// This is asserted by running the wrapper for real from an unrelated directory with a
// relative -queue and checking where the queue file appeared. The text-contract test in
// main_test.go only pins that the resolution calls exist; this one proves the caller's
// location is what they are resolved against, which is the property the operator
// depends on.
func TestOperationalWrapperResolvesPathsBeforeChangingDirectory(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the operational wrapper is a PowerShell script for Windows")
	}
	powershell, err := exec.LookPath("powershell.exe")
	if err != nil {
		t.Skipf("powershell.exe is not available: %v", err)
	}
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "1688-batch-import.ps1"))
	if err != nil {
		t.Fatalf("resolve the wrapper path: %v", err)
	}
	repoRoot := filepath.Dir(filepath.Dir(script))

	scratch := t.TempDir()
	// The relative value deliberately does not exist yet: its creation is the evidence.
	const relativeQueue = "review-round-12/queue.json"
	// The browser is deliberately missing so the run stops before it can open a window;
	// the queue is written first, which is what this test observes.
	args := []string{
		"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script,
		"-Queue", relativeQueue,
		"-Url", "https://detail.1688.com/offer/981645030344.html",
		"-Actor", "actor-1", "-Organization", "org-1",
		"-Browser", "no-such-browser/chrome.exe",
		"-Extension", "no-such-extension",
		"-Profile", "no-such-profile",
	}
	cmd := exec.Command(powershell, args...)
	cmd.Dir = scratch
	output, _ := cmd.CombinedOutput() // a non-zero exit is expected and not the assertion

	want := filepath.Join(scratch, filepath.FromSlash(relativeQueue))
	if _, statErr := os.Stat(want); statErr != nil {
		t.Fatalf("the wrapper did not create the caller-relative queue %s: %v\noutput:\n%s", want, statErr, output)
	}

	// The failure mode this guards against: the same relative path re-based onto the
	// repository root. Clean it up if the bug is present so the worktree stays clean.
	rebased := filepath.Join(repoRoot, filepath.FromSlash(relativeQueue))
	if _, statErr := os.Stat(rebased); statErr == nil {
		defer os.RemoveAll(filepath.Join(repoRoot, "review-round-12"))
		t.Fatalf("the wrapper re-based the caller's relative path onto the repository root: %s exists", rebased)
	}

	// The queue must describe the item the caller asked for, not one chosen by the
	// working directory.
	content, readErr := os.ReadFile(want)
	if readErr != nil {
		t.Fatalf("read the queue the wrapper created: %v", readErr)
	}
	if !strings.Contains(string(content), "981645030344") {
		t.Fatalf("the queue at the caller's location does not describe the requested item:\n%s", content)
	}
}
