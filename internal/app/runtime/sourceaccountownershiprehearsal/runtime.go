// Package sourceaccountownershiprehearsal owns the fixed, disposable B2
// rehearsal command. It has no external-environment or caller-supplied DSN mode.
package sourceaccountownershiprehearsal

import (
	"context"
	"fmt"
	"io"
	"strings"
)

const (
	ExitSuccess             = 0
	ExitInvalidInput        = 2
	ExitAdmissionDenied     = 10
	ExitOutcomeUnknown      = 20
	ExitNotPrepared         = 21
	ExitConflictOrDrift     = 22
	ExitDependencyFailure   = 30
	MaxArgumentBytes        = 64 * 1024
	internalStageFlagPrefix = "--internal-stage="
)

// Run executes the rehearsal CLI without exposing a production operation mode.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if ctx == nil || stdin == nil || stdout == nil || stderr == nil {
		return ExitInvalidInput
	}
	total := 0
	for _, arg := range args {
		total += len(arg)
		if total > MaxArgumentBytes {
			fmt.Fprintln(stderr, "bounded command input required")
			return ExitInvalidInput
		}
	}

	if len(args) == 0 || (len(args) == 1 && args[0] == "inspect") {
		fmt.Fprintln(stderr, "inspect requires a live rehearsal parent")
		return ExitAdmissionDenied
	}
	if len(args) == 1 && strings.HasPrefix(args[0], internalStageFlagPrefix) {
		if !sameExecutableParent() {
			fmt.Fprintln(stderr, "internal stage requires a live rehearsal parent")
			return ExitAdmissionDenied
		}
		return runChildStage(ctx, strings.TrimPrefix(args[0], internalStageFlagPrefix), stdin, stdout, stderr)
	}
	if args[0] != "rehearsal" {
		fmt.Fprintln(stderr, "only inspect or rehearsal is supported")
		return ExitInvalidInput
	}
	scenario := "normal"
	if len(args) == 3 {
		if !strings.HasPrefix(args[2], "--scenario=") {
			fmt.Fprintln(stderr, "only a fixed synthetic rehearsal scenario is supported")
			return ExitInvalidInput
		}
		scenario = strings.TrimPrefix(args[2], "--scenario=")
	}
	if (len(args) != 2 && len(args) != 3) || args[1] != "--yes" || !validRehearsalScenario(rehearsalScenario(scenario)) {
		fmt.Fprintln(stderr, "rehearsal requires explicit --yes")
		return ExitInvalidInput
	}
	return runRehearsal(ctx, rehearsalScenario(scenario), stdout, stderr)
}
