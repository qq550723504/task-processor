package sourceaccountownershiprehearsal

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunDefaultsToInspectAndRejectsWithoutLiveParent(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := Run(context.Background(), nil, strings.NewReader(""), &stdout, &stderr)
	if got != ExitAdmissionDenied {
		t.Fatalf("Run() exit = %d, want %d; stderr=%q", got, ExitAdmissionDenied, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("default inspect wrote stdout without admission: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "live rehearsal parent") {
		t.Fatalf("default inspect error = %q, want live-parent denial", stderr.String())
	}
}

func TestRunRequiresExplicitRehearsalConfirmation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := Run(context.Background(), []string{"rehearsal"}, strings.NewReader(""), &stdout, &stderr)
	if got != ExitInvalidInput {
		t.Fatalf("Run(rehearsal) exit = %d, want %d; stderr=%q", got, ExitInvalidInput, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--yes") {
		t.Fatalf("Run(rehearsal) error = %q, want explicit confirmation", stderr.String())
	}
}

func TestRunRejectsDirectChildAndCopiedStaticGrant(t *testing.T) {
	staticGrant := `{"parent_invocation":"copied","stage":"prepare","nonce":"copied","container_id":"copied","source_dsn":"postgres://shared"}`
	var stdout, stderr bytes.Buffer
	got := Run(context.Background(), []string{"--internal-stage=prepare"}, strings.NewReader(staticGrant), &stdout, &stderr)
	if got != ExitAdmissionDenied {
		t.Fatalf("Run(direct child) exit = %d, want %d; stderr=%q", got, ExitAdmissionDenied, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("direct child wrote protocol output before admission: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "live rehearsal parent") {
		t.Fatalf("direct child error = %q, want live-parent denial", stderr.String())
	}
}

func TestRunRejectsUnknownActionAndOversizedInput(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "unknown", args: []string{"production"}},
		{name: "unexpected rehearsal option", args: []string{"rehearsal", "--yes", "--external=config.json"}},
		{name: "oversized", args: []string{strings.Repeat("x", MaxArgumentBytes+1)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := Run(context.Background(), tc.args, strings.NewReader(""), &stdout, &stderr); got != ExitInvalidInput {
				t.Fatalf("Run(%q) exit = %d, want %d", tc.args[0][:min(len(tc.args[0]), 20)], got, ExitInvalidInput)
			}
		})
	}
}
