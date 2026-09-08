//go:build integration

package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSourceAccountOwnershipRehearsalCommand(t *testing.T) {
	binary := buildRehearsalCommand(t)

	t.Run("default inspect and copied child grant reject", func(t *testing.T) {
		for _, test := range []struct {
			name  string
			args  []string
			stdin string
		}{
			{name: "default inspect"},
			{name: "help", args: []string{"--help"}},
			{name: "direct child", args: []string{"--internal-stage=prepare"}, stdin: `{"container_id":"copied","source_dsn":"postgres://shared"}`},
		} {
			t.Run(test.name, func(t *testing.T) {
				before := rehearsalContainerIDs(t)
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				command := exec.CommandContext(ctx, binary, test.args...)
				command.Stdin = strings.NewReader(test.stdin)
				output, err := command.CombinedOutput()
				wantExit := 10
				if test.name == "help" {
					wantExit = 2
				}
				if exitCode(err) != wantExit {
					t.Fatalf("exit=%d want=%d err=%v output=%s", exitCode(err), wantExit, err, output)
				}
				if test.name != "help" && !strings.Contains(string(output), "live rehearsal parent") {
					t.Fatalf("output=%q", output)
				}
				if after := rehearsalContainerIDs(t); strings.Join(before, ",") != strings.Join(after, ",") {
					t.Fatalf("zero-write command changed rehearsal containers: before=%v after=%v", before, after)
				}
			})
		}
	})

	for _, scenario := range []struct {
		name      string
		arguments []string
		recovered bool
	}{
		{name: "normal", arguments: []string{"rehearsal", "--yes"}},
		{name: "commit response loss", arguments: []string{"rehearsal", "--yes", "--scenario=commit-response-loss"}, recovered: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			before := rehearsalContainerIDs(t)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			command := exec.CommandContext(ctx, binary, scenario.arguments...)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("rehearsal failed: %v\n%s", err, output)
			}
			text := string(output)
			for _, want := range []string{"ISOLATED_REHEARSAL_PASS", "inspect=PASS", "schema_install=PASS", "prepare=PASS", "new_process_receipt_read=PASS", "cleanup=PASS"} {
				if !strings.Contains(text, want) {
					t.Fatalf("output missing %q:\n%s", want, text)
				}
			}
			if strings.Contains(text, "postgres://") {
				t.Fatalf("output leaked a DSN:\n%s", text)
			}
			if scenario.recovered && !strings.Contains(text, "recovered_unknown=PASS") {
				t.Fatalf("response-loss rehearsal did not report recovery:\n%s", text)
			}
			after := rehearsalContainerIDs(t)
			if strings.Join(before, "\n") != strings.Join(after, "\n") {
				t.Fatalf("rehearsal leaked containers: before=%v after=%v", before, after)
			}
		})
	}

	for _, failure := range []struct {
		name     string
		scenario string
		wantExit int
	}{
		{name: "inspect role denied", scenario: "inspect-permission-denied", wantExit: 30},
		{name: "schema role denied", scenario: "schema-permission-denied", wantExit: 30},
		{name: "mapping drift", scenario: "mapping-drift", wantExit: 22},
		{name: "prepare role denied and confirmed absent", scenario: "prepare-permission-denied", wantExit: 21},
		{name: "receipt role denied", scenario: "receipt-permission-denied", wantExit: 20},
		{name: "target drift", scenario: "target-drift", wantExit: 22},
		{name: "unknown with target drift remains unknown", scenario: "commit-response-loss-target-drift", wantExit: 20},
	} {
		t.Run(failure.name, func(t *testing.T) {
			before := rehearsalContainerIDs(t)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			command := exec.CommandContext(ctx, binary, "rehearsal", "--yes", "--scenario="+failure.scenario)
			output, err := command.CombinedOutput()
			if got := exitCode(err); got != failure.wantExit {
				t.Fatalf("exit=%d want=%d err=%v\n%s", got, failure.wantExit, err, output)
			}
			if strings.Contains(string(output), "postgres://") || strings.Contains(string(output), "ISOLATED_REHEARSAL_PASS") {
				t.Fatalf("failed rehearsal leaked a DSN or success claim:\n%s", output)
			}
			after := rehearsalContainerIDs(t)
			if strings.Join(before, ",") != strings.Join(after, ",") {
				t.Fatalf("failed rehearsal leaked task-owned container: before=%v after=%v", before, after)
			}
		})
	}
}

func buildRehearsalCommand(t *testing.T) string {
	t.Helper()
	repoRootBytes, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := strings.TrimSpace(string(repoRootBytes))
	binary := filepath.Join(t.TempDir(), "source-account-ownership-rehearsal")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	command := exec.Command("go", "build", "-o", binary, "./cmd/source-account-ownership-rehearsal")
	command.Dir = repoRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build command: %v\n%s", err, output)
	}
	return binary
}

func rehearsalContainerIDs(t *testing.T) []string {
	t.Helper()
	command := exec.Command("docker", "ps", "-aq", "--filter", "label=task-processor.source-account-b2=issue-364-rehearsal")
	output, err := command.Output()
	if err != nil {
		t.Skipf("Docker unavailable: %v", err)
	}
	text := strings.TrimSpace(string(output))
	if text == "" {
		return nil
	}
	return strings.Fields(text)
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}
